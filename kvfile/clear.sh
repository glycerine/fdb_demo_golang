#!/bin/bash

fdbcli --exec 'writemode on; clearrange \x00 \xff'
fdbcli --exec 'getrange \x00 \xff'

