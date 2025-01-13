#!/bin/bash
set -e

export CC=$(which gcc-12)
export CXX=$(which g++-12)
export CFLAGS="-fopenmp -m64 -mtune=native -march=native -O3 -DNDEBUG"
export CXXFLAGS="-fopenmp -m64 -march=native -mtune=native -O3 -DNDEBUG"
#export LDFLAGS="-ldl -lm -Wl,-fuse-ld=gold -Wl,--as-needed -Wl,--strip-all"
export LDFLAGS="-Wl,-O3 -Wl,--as-needed -Wl,--strip-all"

url="https://github.com/redhat-performance/tuned"
project="redhat-performance/tuned"

get_latest_release() {
  curl --silent "https://api.github.com/repos/$1/releases/latest" | # Get latest release from GitHub api
    grep '"tag_name":' |                                            # Get tag line
    sed -E 's/.*"([^"]+)".*/\1/'                                    # Pluck JSON value
}
version=$(get_latest_release ${project})
echo "The latest version of tuned is ${version}"

DISTRO=$(grep "^NAME=" /etc/os-release | cut -d "=" -f 2 | sed -e 's/^"//' -e's/"$//')
echo "Linux distribution is ${DISTRO}."
if [[ ${DISTRO} == "Ubuntu" ]]; then
    sudo apt -qq update
    sudo apt -y upgrade
    sudo apt install -y git git-core build-essential
elif [[ ${DISTRO} == "Amazon Linux" ]]; then
    sudo dnf update -y
    sudo dnf install -y git pkgconf dnf-plugins-core asciidoc asciidoctor rpm-build \
        desktop-file-utils python3-pyudev python3-gobject python-dbus python3-configobj python3-decorator python-devel
fi

if [ ! -d tuned ]; then
    git clone --depth 1 --branch ${version} ${url}
else
    pushd tuned
    git clean -xdf
    git pull
    popd
fi

pushd tuned
CC=${CC} CXX=${CXX} make rpm
popd

if [ -d "${HOME}/rpmbuild/RPMS/noarch/" ]; then
    ls ${HOME}/rpmbuild/RPMS/noarch/*.rpm
fi

echo "Done."
