#!/bin/bash

fdbcli --exec 'getrange \x00 \xff 50'
