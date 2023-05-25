#!/bin/bash
set -e

source build_catalog.sh
build_a_tag sc-$(date +%Y%m%d)
