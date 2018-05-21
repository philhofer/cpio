# cpio

`cpio(5)` is an archive format similar to `tar(5)`, and is compatible with the `cpio(1)` tool.

## API

The `cpio` package API is almost identical to the `archive/tar` package API from the Go standard library.

## Formats

This library only reads/writes the "newc" archive format. (There are many legacy cpio header formats, but
cpio usage is so unusual these days that implementing all of them would likely be a colosal waste of time.)

You can read/write/list archives of this format using the `cpio(1)` tool.

## Uses

Linux initramfs images are compressed cpio archives, so a cpio library might be useful to you if
you're trying to do unholy things from afar during network boot.
(That was my motivation for writing the package, anyway...)
