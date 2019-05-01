#!/bin/sh
for file in out/binaries/* ; do 
  if [ -e "$file" ]; then
    newname=`echo "$file" | sed 's/amd64/x86_64/; s/386/i686/; s/darwin/Darwin/; s/linux/Linux/; s/freebsd-x86_64/FreeBSD-amd64/; s/arm64/aarch64/;'`
    mv "$file" "$newname"
  fi
done