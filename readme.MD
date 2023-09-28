# ZFS Inplace Recompress

* DISCLAIMER: NO WARRANTY. I TAKE NO RESPONSIBILITY IF THIS DOESN'T WORK, DESTROYS ALL YOUR FILES, EATS YOUR DISKSPACE OR BLOWS UP YOUR COMPUTER. It works for me, and I've tried my best to handle any error I could think of. Test on a small folder with a few files first!

You have a lot of files on your ZFS filesystem, but running 'zfs get compression' tells you that the filesystem is not compressed.

Even if you change this with 'zfs set compression=lz4', that only compresses new blocks of data, not the existing ones.

So what do you do?

You can use zfs-inplace-recompress, which walks all over your filesystem and recompresses files for you, by simultaneously reading and writing from the same files.

Features:
- Rewrites files in-place allowing ZFS to compress blocks
- Has resume support, by using a key-value store to keep track of where you left off
- Multi-threaded for max performance, lets GOOOOOOO
- Preserves last access and modification times
- Handles hardlinked files correctly
- Handles Ctrl-C / SIGINT gracefully

If you're using snapshots on your ZFS filesystems, you should not use this tool, as you will not save any space, as the previous snapshots are immutable and will stay uncompressed. Running this would then use the disk space of the compressed and uncompressed files, which is not what you want.

How do I use this:

```
# zfs get compression
NAME             PROPERTY     VALUE           SOURCE
myfilesystem     compression  off             local

# zfs get compressratio
NAME             PROPERTY       VALUE  SOURCE
myfilesystem     compressratio  1.00x  -

# zfs set compression=lz4 myfilesystem
NAME             PROPERTY     VALUE           SOURCE
myfilesystem     compression  lz4             local

# cd /myfilesystem

# zfs-inplace-recompress [--debug] [--ignore jpg,zip,etc,etc] [--noresume]

# zfs get compressratio
NAME             PROPERTY       VALUE  SOURCE
myfilesystem     compressratio  2.54x  -
```

Profit! 

Mastodon: @lkarlslund@infosec.exchange