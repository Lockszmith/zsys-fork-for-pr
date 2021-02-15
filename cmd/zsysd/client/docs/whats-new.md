Source including images is available at:
    https://didrocks.fr/2020/05/21/zfs-focus-on-ubuntu-20.04-lts-whats-new/
    https://github.com/didrocks/website/blob/master/content/blog/ubuntu-focal-zfs.1.md

# ZFS focus on Ubuntu 20.04 LTS: what’s new?

Ubuntu has supported ZFS as an option for some time. In 19.10, we introduced [experimental support on the desktop](/2019/08/06/ubuntu-zfs-support-in-19.10-introduction/). As explained by then, having a [ZFS on root option on our desktop](/2019/10/11/ubuntu-zfs-support-in-19.10-zfs-on-root/) was only a first step in what we want to achieve by adopting this combined file system and logical volume manager. I strongly suggest you read the 2 above blog posts as introductions to this blog series we are starting. Here we cover what’s new compared to 19.10 in term of installation and general features. We then look at what [ZSys](https://github.com/ubuntu/zsys), our dedicated helper for ZFS systems, can do for you and how you can interact with it. Finally, for the more tech savy, we will deep dive in to how we use ZFS, store properties and understanding how the puzzle fits together. We will give you tips on how to tweak it at your convenience if you are a ZFS sysadmin expert, while still keeping ZSys advanced capabilities compatible.

Without further ado, let’s dive into this!

## ZFS & Ubuntu 20.04 LTS

First thing to note is that our ZFS support with ZSys is still experimental. The installer highlights this in the corresponding screen. With OpenZFS on Ubuntu 20.04 LTS, we are building the first steps of getting the Ubuntu bullet proof desktop.

We hope to be able to drop this experimental support in the coming cycle, and backport this to a 20.04.x release.

## OpenZFS Version

We updated from 19.10 to 20.04 LTS OpenZFS to its latest and greatest available release (0.8.3). As usual, those releases ([0.8.2](https://github.com/openzfs/zfs/releases/tag/zfs-0.8.2) and [0.8.3](https://github.com/openzfs/zfs/releases/tag/zfs-0.8.2)) bring a lot of improvements and fixes (note though that we backported some fixes in Ubuntu 19.10 from 0.8.2 into our package to fix some critical issues for our users). We also backported in Ubuntu 20.04 LTS other fixes in our kernel from the incoming 0.8.4 (and OpenZFS 2.0) release, like encryption performance enhancements. ZSys and other components (for instance ZFS bindings) have been updated to work with the new libzfs version.

Thanks to this, we are committed to deliver the best OpenZFS on linux experience to our audience by continuing to fix any important issues that arise and backporting any critical fixes from newer releases.

## More streamline desktop with ZFS on root installation

The installer experience has been reworked to be clearer and offer more focused options for the user. Installing ZFS on root has never been simpler, especially if you compare to the excellent, but very long and manual [How-To Ubuntu Root on ZFS](https://github.com/openzfs/zfs/wiki/Ubuntu-18.04-Root-on-ZFS) created by the OpenZFS community.

We support, for now, only full disk setup. The options related to this (LVM, LVM+encryption and ZFS) are all under the "Advanced features" screen.

![Press Advanced features in Erase disk and install ubuntu](/images/focal/zfs_install_step1.png)

You will find there the Experimental ZFS support option.

![Experimental ZFS support options screen](/images/focal/zfs_install_step2.png)

"ZFS selected" should now be displayed.

![ZFS selected screen](/images/focal/zfs_install_step3.png)

If you have multiple disks available, an additional screen will present the available choices. Once validated, a recap of the different partitions that we are going to create is waiting upon your approval.

![Recap of partitions to create](/images/focal/zfs_install_step4.png)

In a later blog posts, we will see why and what all those partitions are about, their sizes and various decisions which lead to that layout.

The installation will then proceed as usual. Upon reboot, your newly installed Ubuntu 20.04 LTS system will be powered with ZFS on root!

Here are some proofs (but you don’t need to type those commands):

![ZFS is powering your system and ZSys is active](/images/focal/zfs_install_done.png)

And that’s it! For your daily driver, you don’t need to do anything more. Unless you want to understand a little bit more what this will bring to you in addition to all the ZFS robustness, checksuming, decompression and such, this is where you can stop! The system will work silently for you and you can forget about it. :)

## Pool enhancements

For technical savy users, they will be delighted to know that we upgraded the **bpool** version to 5000 (previously, it was at version 28). We created - only on **bpool** - a selection of features to ensure that grub will be able to read and boot from any ZFS filesystem or snapshot datasets.

However, `zpool` command line users were encouraged then to upgrade the pool to enable new features on it, those being incompatible with grub! Worse, once the pool was upgraded, it was impossible to downgrade and your system was basically broken. Seeing this happening and until there is a dedicated version-by-components (discussions started upstream), we removed this message on **bpool** from `zpool status` and prevented to upgrade is with `zpool upgrade`. **rpool** doesn’t have this limitation and have a bunch of features enabled.

## Boot enhancements

With ZSys installed, building a boot grub menu sees some great enhancements. We basically divided by 4 the time to generate this menu thanks to multiple profiling technics, which will make any operation needing to rebuild it way more enjoyable.

Building the menu is something, but starting grub is another. With a number of state saves, you can rapidly hit the score of more than a hundred of them. Under those conditions, we saw performance degradation in navigating grub menus. In particular, displaying the first menu was taking 14s and navigating the history of state was taking more than 80s! We made some drastic enhancements there by making both of them **instantaneous**! We had to come with multiple -hem- """creative""" strategies for fixing that one (reducing `grub.cfg` from 7329 lines that grub didn’t really like to 728, each new history entry being 3 additional lines instead of 50 :p). We crafted all this to still be readable for advanced users who want to manually edit the boot command line before starting a system.

Similarly, we were able to significantly speed up the boot time and make the systemd ZFS mount generator more robust: basically, unless you revert to another system state, ZSys will be out of the way, not taking any additional time on the critical boot path. Also, if some pools to import are not in a coherent status, you will be dropped to system emergency state, asking you to fix this before booting. However, this should only be the case of a failing revert, and booting in the current, last known good state, should work. This is an area we will continue to work on, of course.

Finally, we fixed a bunch of issues on both grub and boot chainload that people testing ZFS on 19.10 reported to us, like external pools being systematically exported, some configurations issues and more. Thanks to everyone here, it helps to deliver a more solid experience with ZFS on Ubuntu 20.04 LTS!

## ZSys is now installed by default

Some Ubuntu flavors already shipped it in 19.10, but ZSys has seen a number of changes and new capabilities. After a security review, it is now seeded by default on the Ubuntu desktop 20.04 LTS installation!

There are a lot of things to discuss about this component in term of features and what it exactly brings to the user. Not only does this allow for system revert (and grub bootloader will now present them), but it is not limited to being a boot environment. In fact, there is so much to talk about that we will dedicate the next four blog posts entirely to it.

The good news is that most of the features are completely working under the hood and transparently to the user. However, if you need to revert an upgrade, get back old files from an user, or are just interested in it, those posts should be of interest to you! See you there :)

Meanwhile, join the discussion via the [dedicated Ubuntu discourse thread](https://discourse.ubuntu.com/t/zfs-focus-on-ubuntu-20-04-lts-blog-posts/16355).
