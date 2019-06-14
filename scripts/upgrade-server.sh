#! /bin/bash
###############################################################################
##											writefreely update script														 ##
##																																					 ##
##		WARNING: running this script will overwrite any modifed assets or 		 ##
##		template files. If you have any custom changes to these files you 		 ##
##		should back them up FIRST.																						 ##
##																																					 ##
##		This must be run from the web application root directory							 ##
##		i.e. /var/www/writefreely, and operates under the assumption that you  ##
##		have not installed the binary `writefreely` in another location.			 ##
###############################################################################
#
#			Copyright © 2019 A Bunch Tell LLC.
#
#			This file is part of WriteFreely.
#
#			WriteFreely is free software: you can redistribute it and/or modify
#			it under the terms of the GNU Affero General Public License, included
#			in the LICENSE file in this source code package.
#


# only execute as root, or use sudo

if [[ `id -u` -ne 0 ]]; then
	echo "You must login as root, or execute this script with sudo"
	exit 10
fi

# go ahead and check for the latest release on linux
echo "Checking for updates.."

url=`curl -s https://api.github.com/repos/writeas/writefreely/releases/latest | grep 'browser_' | grep linux | cut -d\" -f4`

# check current version

current=`./writefreely -v`

echo "Current version is v${current:(-5):5}"

# grab latest version number
IFS='/'
read -ra parts <<< "$url"

latest=${parts[-2]}
echo "Latest release is $latest"


IFS='.'
read -ra cv <<< "${current:(-5):5}"
read -ra lv <<< "${latest#v}"

IFS=' '
tempdir=$(mktemp -d)


if [[ ${lv[0]} -gt ${cv[0]} ]]; then
	echo "New major version"
	echo "Downloading.."
	`wget -P $tempdir -q --show-progress $url`
elif [[ ${lv[0]} -eq ${cv[0]} ]] && [[ ${lv[1]} -gt ${cv[1]} ]]; then
	echo "New minor version"
	echo "Downloading.."
	`wget -P $tempdir -q --show-progress $url`
elif [[ ${lv[2]} -gt ${cv[2]} ]]; then
	echo "New patch version"
	echo "Downloading.."
	`wget -P $tempdir -q --show-progress $url`
else
	echo "Nothing to change"
	exit 0
fi

filename=${parts[-1]}

# extract
echo "Extracing files.."
tar -zxf $tempdir/$filename -C $tempdir

# copy files
echo "Copying files.."
cp -r $tempdir/{pages,static,templates,writefreely} .

# restart service
echo "Restarting writefreely systemd service.."
`systemctl restart writefreely`
echo "Done, version has been upgraded to $latest."
