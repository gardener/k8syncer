#!/bin/bash
#
# SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors.
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

PROJECT_ROOT="$(realpath $(dirname $0)/..)"
DOCS_FOLDER="${PROJECT_ROOT}/docs"
METAFILE_NAME=".docnames"

if [[ -z ${LOCALBIN:-} ]]; then
  LOCALBIN="$PROJECT_ROOT/bin"
fi
if [[ -z ${JQ:-} ]]; then
  JQ="$LOCALBIN/jq"
fi

doc_index_file=${1:-"$DOCS_FOLDER/README.md"}

# prints to the new doc index
function println() {
  echo "$@" >> "$newindex"
}

# expects a path to a folder as argument
# returns how this folder should be named in an index
function getDocFolderName() {
  local metafile="$1/$METAFILE_NAME"
  if [[ -f "$metafile" ]]; then
    cat "$metafile" | $JQ -r '.header'
  fi
}

# expects two arguments:
# - path to a doc folder
# - name of the file in there
# the file is expected to contain its header in the first line
# or there should be an overwrite present in <foldername>/$METAFILE_NAME
function getDocName() {
  local metafile="$1/$METAFILE_NAME"
  local filename="$2"
  if [[ -f "$metafile" ]]; then
    local overwrite="$(cat "$metafile" | $JQ -r '.overwrites[$name]' --arg name "$2")"
    if [[ "$overwrite" != "null" ]]; then
      echo "$overwrite"
      return
    fi
  fi
  if [[ -f "$filename" ]]; then
    local firstline=$(cat "$filename" | head -n 1)
    echo "${firstline#'# '}"
  fi
}

echo "> Generating Documentation Index"

newindex=$(mktemp)

println '<!-- Do not edit this file, as it is auto-generated!-->'
println "# Documentation Index"
println

(
  cd "$DOCS_FOLDER"
  for f in *; do 
    if [[ -d "$f" ]]; then
      foldername="$(getDocFolderName "$f")"
      if [[ -z "$foldername" ]]; then
        echo "Ignoring folder '$f' due to missing '$METAFILE_NAME' file."
        continue
      fi

      println "## $foldername"
      println

      (
        cd "$f"
        for f2 in *.md; do
          docname="$(getDocName "../$f" "$f2")"
          if [[ -z "$docname" ]]; then
            echo "Ignoring file '$f/$f2' because the header could not be determined."
            # There are two possible reasons for this:
            # 1. The file doesn't start with a '# <headline>' in the first line and no overwrite is defined in the folder's metafile.
            # 2. The overwrite in the folder's metafile explicitly sets the name to an empty string, meaning this file should be ignored.
            continue
          fi
          println "- [$docname]($f/$f2)"
        done
      )

      println
    fi
  done
)

cp "$newindex" "$doc_index_file"