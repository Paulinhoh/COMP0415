#
# Poxim-V Python simulator example
# 
# (C) Copyright 2024 Bruno Otavio Piedade Prado
#
# This file is part of Poxim-V.
#
# Poxim-V is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# Poxim-V is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with Poxim-V.  If not, see <https://www.gnu.org/licenses/>.
#

# How to run:
# python nomesobrenome_123456789012_exemplo.py input.hex output.out

# System library
import sys

# Main function
def main(argv):
    # Outputting separator
    print("--------------------------------------------------------------------------------")
    # Iterating over arguments
    for i, argvi in enumerate(argv):
        # Outputting argument
        print("argv[%u] = %s" % (i, argvi))
    # Opening input and output files using proper permissions
    # input = open(sys.args[1], "r")
    # output = open(sys.args[2], "w")
    # .
    # .
    # .
    # Closing input and output files
    #input.close()
    #output.close()
    # Outputting separator
    print("--------------------------------------------------------------------------------")

# Python start point
if __name__ == "__main__":
    # Call main function with command line arguments
    main(sys.argv)