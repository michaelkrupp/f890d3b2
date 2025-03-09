GAWK     ?= gawk
GIT      ?= git
TTY      ?= tty

NOOP=
SPACE=$(NOOP) $(NOOP)
COMMA=,

SHELL := /usr/bin/env bash
SHELL += -eu -o pipefail

.DELETE_ON_ERROR:
.SUFFIXES:

VERBOSE := $(if $(value CI),1)

ifneq "" "$(VERBOSE)"
 $(warning ***** starting Makefile for goal(s) "$(MAKECMDGOALS)")
 $(warning ***** $(shell $(DATE)) on $(shell $(UNAME) -a))

 MAKEFLAGS += --print-directory
 MAKEFLAGS += --trace
 MAKEFLAGS += --debug=a
else
 MAKEFLAGS += --no-print-directory
endif

MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-keep-going
MAKEFLAGS += --no-builtin-rules --no-builtin-variables

HAS_TTY := $(if $(shell $(TTY) -s && echo 1),1)

ifneq "" "$(HAS_TTY)"
 .DEFAULT_GOAL := .usage
else
 .DEFAULT_GOAL := build
endif

.PHONY: help ## Show this help page
.NOTPARALLEL: help
help: export LC_ALL=C
help:
	@ $(GAWK) -f "$(dir $(firstword $(MAKEFILE_LIST)))/makefile.awk" \
    -v NAME="$(notdir $(realpath $(dir $(firstword $(MAKEFILE_LIST)))))" \
    -v GIT="$(GIT)" \
    -v MAKE="$(MAKE)" \
    -v MAKEFILE="$(firstword $(MAKEFILE_LIST))" \
  -- \
  $(MAKEFILE_LIST)

.PHONY: .usage
.NOTPARALLEL: .usage
.usage: help
	@ echo >&2
	@ exit 1
