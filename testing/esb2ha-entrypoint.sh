#!/bin/sh
case "$1" in
  # If it starts with / let's assume it is a full path to run.
  (/*) exec "$@" ;;
  # Otherwise it is a parameter for esb2ha
  (*)  exec esb2ha "$@" ;;
esac
