#!/bin/bash
# Create a manpage from the README.md

# Add header
echo '''% debos(1)

# NAME

    debos -  Debian OS images builder
''' > debos.md

# Add README.md
tail -n +2 ../../README.md >> debos.md

# Some tweaks to the markdown
# Uppercase titles
sed -i 's/^\(##.*\)$/\U\1/' debos.md

# Remove double #
sed -i 's/^\##/#/' debos.md

# Create the manpage
pandoc -s -t man debos.md -o debos.1

# Resulting manpage can be browsed with groff:
#groff -man -Tascii debos.1


