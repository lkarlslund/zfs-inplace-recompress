# ZFS Inplace Recompress

You have a lot of files on your ZFS filesystem, but running 'zfs get compression' tells you that the filesystem is not compressed.

Even if you change this with 'zfs set compression=lz4', that only compresses new blocks of data, not the existing ones.

So what do you do?

You can use zfs-inplace-recompress, which walks all over your filesystem and recompresses files for you, by simultaneously reading and writing from the same files.

Features:
- Compresses files in-place
- Has resume support, by using a key-value store to keep track of where you left off
- Handles hardlinked files correctly

If you're using snapshots on your ZFS filesystems, you should not use this tool, as you will not save any space, as the previous snapshots are immutable and will stay uncompressed. Running this would then use the disk space of the compressed and uncompressed files, which is not what you want.

How do I use this:

```
# zfs-inplace-recompress [--debug] [--ignoreext abc,def] [--parallel N] [--noresume]
```
