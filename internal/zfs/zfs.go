package zfs

import (
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/ubuntu/zsys/internal/config"

	libzfs "github.com/bicomsystems/go-libzfs"
	"golang.org/x/xerrors"
)

const (
	zsysPrefix = "org.zsys:"
	// BootfsProp string value
	BootfsProp = zsysPrefix + "bootfs"
	// LastUsedProp string value
	LastUsedProp = zsysPrefix + "last-used"
	// SystemDataProp string value
	SystemDataProp = zsysPrefix + "system-dataset"
	// CanmountProp string value
	CanmountProp = "canmount"
)

/*
destroy
rename
promote
properties write:
	- canmount (when changing current dataset)
	- org.zsys:bootfs (when cloning)
	- org.zsys:last-used (every boot)
  - org.zsys:system-dataset

*/

/*// CanMountState represents the different state of CanMount that the dataset can have.
type CanMountState int

const (
	// CanMountOff is canmount=off.
	CanMountOff CanMountState = iota
	// CanMountAuto is canmount=auto.
	CanMountAuto
	// CanMountOn is canmount=on.
	CanMountOn
)*/

// Dataset is the abstraction of a physical dataset and exposes only properties that must are accessible by the user.
type Dataset struct {
	// Name of the dataset.
	Name string
	// IsSnapshot tells if the dataset is a snapshot
	IsSnapshot bool
	DatasetProp
}

// DatasetProp abstracts some properties for a given dataset
type DatasetProp struct {
	// Mountpoint where the dataset will be mounted (without alt-root).
	Mountpoint string
	// CanMount state of the dataset.
	CanMount string
	// BootFS is a user property stating if the dataset should be mounted in the initramfs.
	BootFS string
	// LastUsed is a user property that store the last time a dataset was used.
	LastUsed int
	// SystemDataset is a user proper for user datasets, linking them to relevant system dataset.
	SystemDataset string

	// Here are the sources (not exposed to the public API) for each property
	// Used mostly for tests
	sources datasetSources
}

// datasetSources list sources some properties for a given dataset
type datasetSources struct {
	Mountpoint    string
	CanMount      string
	BootFS        string
	LastUsed      string
	SystemDataset string
}

// Zfs is a system handler talking to zfs linux module.
// It can handle a single transaction if "WithTransaction()"" is passed to the New constructor.
// An error won't then try to rollback the changes and Cancel() should be called.
// If no error happened and we want to finish the transaction before starting a new one, call "Done()".
// If no transaction support is used, any error in a method call will try to rollback changes automatically.
type Zfs struct {
	transactional  bool
	reverts        []func() error
	transactionErr bool
}

// New return a new zfs system handler.
func New(options ...func(*Zfs)) *Zfs {
	z := Zfs{}
	for _, options := range options {
		options(&z)
	}

	return &z
}

// Scan returns all datasets that are currently imported on the system.
func (Zfs) Scan() ([]Dataset, error) {
	ds, err := libzfs.DatasetOpenAll()
	if err != nil {
		return nil, xerrors.Errorf("can't list datasets: %v", err)
	}
	defer libzfs.DatasetCloseAll(ds)

	var datasets []Dataset
	for _, d := range ds {
		datasets = append(datasets, collectDatasets(d)...)
	}

	return datasets, nil
}

// getDatasetsProp returns all properties for a given dataset and the source of them.
// for snapshots, we'll take the parent dataset for the mount properties.
func getDatasetProp(d libzfs.Dataset) (*DatasetProp, error) {
	sources := datasetSources{}
	name := d.Properties[libzfs.DatasetPropName].Value

	var mountPropertiesDataset = &d
	if d.IsSnapshot() {
		parentName := name[:strings.LastIndex(name, "@")]
		pd, err := libzfs.DatasetOpen(parentName)
		if err != nil {
			return nil, xerrors.Errorf("can't get parent dataset: "+config.ErrorFormat, err)
		}
		defer pd.Close()
		mountPropertiesDataset = &pd
	}

	var mountpoint, canMount string
	mp, err := mountPropertiesDataset.GetProperty(libzfs.DatasetPropMountpoint)
	if err != nil {
		return nil, xerrors.Errorf("can't get mountpoint: "+config.ErrorFormat, err)
	}
	sources.Mountpoint = mp.Source

	p, err := mountPropertiesDataset.Pool()
	if err != nil {
		return nil, xerrors.Errorf("can't get associated pool: "+config.ErrorFormat, err)
	}
	poolRoot, err := p.GetProperty(libzfs.PoolPropAltroot)
	if err != nil {
		return nil, xerrors.Errorf("can't get altroot for associated pool: "+config.ErrorFormat, err)
	}
	mountpoint = strings.TrimPrefix(mp.Value, poolRoot.Value)
	if mountpoint == "" {
		mountpoint = "/"
	}

	cm, err := mountPropertiesDataset.GetProperty(libzfs.DatasetPropCanmount)
	if err != nil {
		return nil, xerrors.Errorf("can't get canmount property: "+config.ErrorFormat, err)
	}
	canMount = cm.Value
	sources.CanMount = cm.Source

	bfs, err := d.GetUserProperty(BootfsProp)
	if err != nil {
		return nil, xerrors.Errorf("can't get bootfs property: "+config.ErrorFormat, err)
	}
	bootfs := bfs.Value
	if bootfs == "-" {
		bootfs = ""
	}
	sources.BootFS = bfs.Source

	var lu libzfs.Property
	if !d.IsSnapshot() {
		lu, err = d.GetUserProperty(LastUsedProp)
		if err != nil {
			return nil, xerrors.Errorf("can't get %q property: "+config.ErrorFormat, LastUsedProp, err)
		}
	} else {
		lu, err = d.GetProperty(libzfs.DatasetPropCreation)
		if err != nil {
			return nil, xerrors.Errorf("can't get creation property: "+config.ErrorFormat, err)
		}
	}
	sources.LastUsed = lu.Source
	if lu.Value == "-" {
		lu.Value = "0"
	}
	lastused, err := strconv.Atoi(lu.Value)
	if err != nil {
		return nil, xerrors.Errorf("%q property isn't an int: "+config.ErrorFormat, LastUsedProp, err)
	}

	sDataset, err := d.GetUserProperty(SystemDataProp)
	if err != nil {
		return nil, xerrors.Errorf("can't get %q property: "+config.ErrorFormat, SystemDataProp, err)
	}
	systemDataset := sDataset.Value
	if systemDataset == "-" {
		systemDataset = ""
	}
	sources.SystemDataset = sDataset.Source

	return &DatasetProp{
		Mountpoint:    mountpoint,
		CanMount:      canMount,
		BootFS:        bootfs,
		LastUsed:      lastused,
		SystemDataset: systemDataset,
		sources:       sources,
	}, nil
}

// collectDatasets returns a Dataset tuple of all its properties and children
func collectDatasets(d libzfs.Dataset) []Dataset {
	var results []Dataset
	var collectErr error

	defer func() {
		if collectErr != nil {
			log.Printf("couldn't load dataset: "+config.ErrorFormat+"\n", collectErr)
		}
	}()

	// Skip non file system or snapshot datasets
	if d.Type == libzfs.DatasetTypeVolume || d.Type == libzfs.DatasetTypeBookmark {
		return nil
	}

	name := d.Properties[libzfs.DatasetPropName].Value

	props, err := getDatasetProp(d)
	if err != nil {
		collectErr = xerrors.Errorf("can't get dataset properties for %q: "+config.ErrorFormat, name, collectErr)
		return nil
	}

	results = append(results,
		Dataset{
			Name:        name,
			IsSnapshot:  d.IsSnapshot(),
			DatasetProp: *props,
		})

	for _, dc := range d.Children {
		results = append(results, collectDatasets(dc)...)
	}

	return results
}

// Snapshot creates a new snapshot for dataset (and children if recursive is true) with the given name.
func (Zfs) Snapshot(snapName, datasetName string, recursive bool) (errSnapshot error) {
	d, err := libzfs.DatasetOpen(datasetName)
	if err != nil {
		return xerrors.Errorf("couldn't open %q: %v", datasetName, err)
	}
	defer d.Close()

	// We can't use the recursive version of snapshotting, as we want to track user properties and
	// set them explicitely as needed

	// Cleanup newly created snapshot datasets if we or a children returns an error (like intermediate datasets have a snapshot)
	var snapshotDatasetNames []string
	defer func() {
		if snapshotDatasetNames == nil || errSnapshot == nil {
			return
		}
		// start with leaves to undo clone creation
		sort.Sort(sort.Reverse(sort.StringSlice(snapshotDatasetNames)))
		for _, n := range snapshotDatasetNames {
			d, err := libzfs.DatasetOpen(n)
			if err != nil {
				log.Printf("couldn't open %q for cleanup: %v", n, err)
				continue
			}
			defer d.Close()
			if err := d.Destroy(false); err != nil {
				log.Printf("couldn't destroy %q for cleanup: %v", n, err)
			}
		}
	}()

	// recursively try snapshotting all children. If an error is returned, the closure will clean newly created datasets.
	var snapshotInternal func(libzfs.Dataset) error
	snapshotInternal = func(d libzfs.Dataset) error {
		datasetName := d.Properties[libzfs.DatasetPropName].Value

		// Get properties from parent snapshot.
		srcProps, err := getDatasetProp(d)
		if err != nil {
			return xerrors.Errorf("can't get dataset properties for %q: "+config.ErrorFormat, datasetName, err)
		}

		props := make(map[libzfs.Prop]libzfs.Property)
		snapshotDatasetName := datasetName + "@" + snapName
		ds, err := libzfs.DatasetSnapshot(snapshotDatasetName, false, props)
		if err != nil {
			return xerrors.Errorf("couldn't snapshot %q: %v", datasetName, err)
		}
		snapshotDatasetNames = append(snapshotDatasetNames, snapshotDatasetName)
		defer ds.Close()

		// Set user properties that we couldn't set before creating the snapshot dataset.
		// We don't set LastUsed here as Creation time will be used.
		if srcProps.sources.BootFS == "local" {
			if err := ds.SetUserProperty(BootfsProp, srcProps.BootFS); err != nil {
				return xerrors.Errorf("couldn't set user property %q to %q: "+config.ErrorFormat, BootfsProp, snapshotDatasetName, err)
			}
		}
		if srcProps.sources.SystemDataset == "local" {
			if err := ds.SetUserProperty(SystemDataProp, srcProps.SystemDataset); err != nil {
				return xerrors.Errorf("couldn't set user property %q to %q: "+config.ErrorFormat, SystemDataProp, snapshotDatasetName, err)
			}
		}

		if !recursive {
			return nil
		}

		// Take snapshots on non snapshot dataset children
		for _, cd := range d.Children {
			if cd.IsSnapshot() {
				continue
			}
			if err := snapshotInternal(cd); err != nil {
				return err
			}
		}
		return nil
	}

	return snapshotInternal(d)
}

// Clone creates a new dataset from a snapshot (and children if recursive is true) with a given suffix,
// stripping older _<suffix> if any.
func (z Zfs) Clone(name, suffix string, recursive bool) (errClone error) {

	if suffix == "" {
		return xerrors.Errorf("no suffix was provided for cloning")
	}

	// Cleanup newly created datasets if we or a children returns an error (like intermediate datasets have a snapshot)
	var clonedDatasetNames []string
	defer func() {
		if clonedDatasetNames == nil || errClone == nil {
			return
		}
		// start with leaves to undo clone creation
		sort.Sort(sort.Reverse(sort.StringSlice(clonedDatasetNames)))
		for _, n := range clonedDatasetNames {
			d, err := libzfs.DatasetOpen(n)
			if err != nil {
				log.Printf("couldn't open %q for cleanup: %v", n, err)
				return
			}
			defer d.Close()
			if err := d.Destroy(false); err != nil {
				log.Printf("couldn't destroy %q for cleanup: %v", n, err)
			}
		}
	}()

	d, err := libzfs.DatasetOpen(name)
	if err != nil {
		return xerrors.Errorf("%q doesn't exist: %v", name, err)
	}
	defer d.Close()

	if !d.IsSnapshot() {
		return xerrors.Errorf("%q isn't a snapshot", name)
	}

	rootName, snaphotName := separateSnaphotName(name)

	// Reformat the name with the new uuid and clone now the dataset.
	newRootName := rootName
	suffixIndex := strings.LastIndex(newRootName, "_")
	if suffixIndex != -1 {
		newRootName = newRootName[:suffixIndex]
	}
	newRootName += "_" + suffix

	// recursively try cloning all children. If an error is returned, the closure will clean newly created datasets.
	var cloneInternal func(libzfs.Dataset) error
	cloneInternal = func(d libzfs.Dataset) error {
		name := d.Properties[libzfs.DatasetPropName].Value

		// Get properties from snapshot and parents.
		srcProps, err := getDatasetProp(d)
		if err != nil {
			return xerrors.Errorf("can't get dataset properties for %q: "+config.ErrorFormat, name, err)
		}

		props := make(map[libzfs.Prop]libzfs.Property)
		if srcProps.sources.Mountpoint == "local" {
			props[libzfs.DatasetPropMountpoint] = libzfs.Property{
				Value:  srcProps.Mountpoint,
				Source: srcProps.sources.Mountpoint,
			}
		}
		if srcProps.sources.CanMount == "local" {
			if srcProps.CanMount == "on" {
				// don't mount new cloned dataset on top of parent.
				srcProps.CanMount = "noauto"
			}
			props[libzfs.DatasetPropCanmount] = libzfs.Property{
				Value:  srcProps.CanMount,
				Source: srcProps.sources.CanMount,
			}
		}

		datasetRelPath := strings.TrimPrefix(strings.TrimSuffix(name, "@"+snaphotName), rootName)
		n := newRootName + datasetRelPath
		cd, err := d.Clone(n, props)
		if err != nil {
			return xerrors.Errorf("couldn't clone %q to %q: "+config.ErrorFormat, name, n, err)
		}
		clonedDatasetNames = append(clonedDatasetNames, n)

		// Set user properties that we couldn't set before creating the dataset. Based this for local
		// or source == parentName (as it will be local)
		parentName := name[:strings.LastIndex(name, "@")]
		if srcProps.sources.BootFS == "local" || srcProps.sources.BootFS == parentName {
			if err := cd.SetUserProperty(BootfsProp, srcProps.BootFS); err != nil {
				return xerrors.Errorf("couldn't set user property %q to %q: "+config.ErrorFormat, BootfsProp, n, err)
			}
		}
		if srcProps.sources.SystemDataset == "local" || srcProps.sources.SystemDataset == parentName {
			if err := cd.SetUserProperty(SystemDataProp, srcProps.SystemDataset); err != nil {
				return xerrors.Errorf("couldn't set user property %q to %q: "+config.ErrorFormat, SystemDataProp, n, err)
			}
		}
		// We don't set LastUsed in purpose as the dataset isn't used yet

		if !recursive {
			return nil
		}

		// Handle other datasets (children of parent) which may have snapshots
		parent, err := libzfs.DatasetOpen(parentName)
		if err != nil {
			return xerrors.Errorf("can't get parent dataset of %q: "+config.ErrorFormat, name, err)
		}
		defer parent.Close()

		for _, cd := range parent.Children {
			if cd.IsSnapshot() {
				continue
			}
			// Look for childrens filesystem datasets having a corresponding snapshot
			found, snapD := cd.FindSnapshotName("@" + snaphotName)
			if !found {
				continue
			}

			if err := cloneInternal(snapD); err != nil {
				return err
			}
		}

		return nil
	}

	// Integrity checks, there are multiple cases:
	// - All children datasets with a snapshot with the same name exists -> OK, nothing in particular to deal with
	// - One dataset doesn't have a snapshot with the same name:
	//   * If no of its children of this dataset has a snapshot with the same name:
	//     * the dataset (and its children) has been created after the snapshot was taken -> OK
	//     * the dataset snapshot (and all its children snapshots) have been removed entirely: no way to detect the difference from above -> consider OK
	//   * If one of its children has a snapshot wi		h the same name: clearly a case where something went wrong during snapshot -> error OUT
	// Said differently:
	// if a dataset has a snapshot with a given, all its parents should have a snapshot with the same name (up to base snapshotName)
	var checkElemAndChildren func(libzfs.Dataset, bool) error
	checkElemAndChildren = func(d libzfs.Dataset, snapshotExpected bool) error {
		found, _ := d.FindSnapshotName("@" + snaphotName)

		// No more snapshot was expected for children (parent dataset didn't have a snapshot, so all children shouldn't have them)
		if found && !snapshotExpected {
			name := d.Properties[libzfs.DatasetPropName].Value
			return xerrors.Errorf("parent of %q doesn't have a snapshot named %q. Every of its children shouldn't have a snapshot. However %q exists.",
				name, snaphotName, name+"@"+snaphotName)
		}

		for _, cd := range d.Children {
			if err := checkElemAndChildren(cd, found); err != nil {
				return err
			}
		}
		return nil
	}

	parent, err := libzfs.DatasetOpen(d.Properties[libzfs.DatasetPropName].Value[:strings.LastIndex(name, "@")])
	if err != nil {
		return xerrors.Errorf("can't get parent dataset of %q: "+config.ErrorFormat, name, err)
	}
	defer parent.Close()
	if err := checkElemAndChildren(parent, true); err != nil {
		return xerrors.Errorf("integrity check failed: %v", err)
	}
	return cloneInternal(d)
}

// SetProperty to given dataset if it was a local/none/snapshot directly inheriting from parent value.
// force does it even if the property was inherited.
// For zfs properties, only a fix set is supported. Right now: "canmount"
func (Zfs) SetProperty(name, value, datasetName string, force bool) error {
	d, err := libzfs.DatasetOpen(datasetName)
	if err != nil {
		return xerrors.Errorf("can't get dataset %q: "+config.ErrorFormat, datasetName, err)
	}
	defer d.Close()

	var parentName string
	if d.IsSnapshot() {
		parentName = datasetName[:strings.LastIndex(datasetName, "@")]
	}
	var prop libzfs.Property
	if !strings.Contains(name, ":") {
		var propName libzfs.Prop
		switch name {
		case CanmountProp:
			propName = libzfs.DatasetPropCanmount
		default:
			return xerrors.Errorf("can't set unsupported property %q for %q", name, datasetName)
		}
		prop, err = d.GetProperty(propName)
		if err != nil {
			return xerrors.Errorf("can't get dataset property %q for %q: "+config.ErrorFormat, name, datasetName, err)
		}
		if !force && prop.Source != "local" && prop.Source != "none" && prop.Source != parentName {
			return nil
		}
		if err = d.SetProperty(propName, value); err != nil {
			return xerrors.Errorf("can't set dataset property %q=%q for %q: "+config.ErrorFormat, name, value, datasetName, err)
		}
		return nil
	}

	// User properties
	prop, err = d.GetUserProperty(name)
	if err != nil {
		return xerrors.Errorf("can't get dataset user property %q for %q: "+config.ErrorFormat, name, datasetName, err)
	}
	if !force && prop.Source != "local" && prop.Source != "none" && prop.Source != parentName {
		return nil
	}
	if err = d.SetUserProperty(name, value); err != nil {
		return xerrors.Errorf("can't set dataset user property %q=%q for %q: "+config.ErrorFormat, name, value, datasetName, err)
	}

	return nil
}

// getSnapName return base and trailing names
func separateSnaphotName(name string) (string, string) {
	i := strings.LastIndex(name, "@")
	if i < 0 {
		return name, ""
	}
	return name[:i], name[i+1:]
}
