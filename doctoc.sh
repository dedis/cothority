#!/bin/bash

############
#
# Motivation to rewrite a simple alternative to doctoc: How Much Do You Trust That Package? Understanding The Software Supply Chain https://www.youtube.com/watch?v=fnELtqE6mMM
#
# Attribution: this include includes prework [done by meleu](https://gist.github.com/meleu/57867f4a01ede1bd730f14b2f018ae89) and I developed the way doctoc interacts with markdown files
#
# Generates a Table of Contents getting a markdown file as input.
#
# Inspiration for this script:
# https://medium.com/@acrodriguez/one-liner-to-generate-a-markdown-toc-f5292112fd14
#
# The list of invalid chars is probably incomplete, but is good enough for my
# current needs.
# Got the list from:
# https://github.com/thlorenz/anchor-markdown-header/blob/56f77a232ab1915106ad1746b99333bf83ee32a2/anchor-markdown-header.js#L25
#

INVALID_CHARS="'[]/?!:\`.,()*\";{}+=<>~$|#@&–—"

check() {

    # src https://stackoverflow.com/questions/6482377/check-existence-of-input-argument-in-a-bash-shell-script
    if [ -z "$1" ]; then
        echo "Error. No argument found. Put as argument a file.md"
        exit 1
    fi

    [[ ! -f "$1" ]] && echo "Error. File not found" && exit

}

toc() {

    local line
    local level
    local title
    local anchor
    local output

    while IFS='' read -r line || [[ -n "$line" ]]; do
        level="$(echo "$line" | sed -E 's/^#(#+).*/\1/; s/#/  /g; s/^  //')"
        title="$(echo "$line" | sed -E 's/^#+ //')"
        anchor="$(echo "$title" | tr '[:upper:] ' '[:lower:]-' | tr -d "$INVALID_CHARS")"

        # check new line introduced is not duplicated, if is duplicated, introduce a number at the end
        #   copying doctoc behavior
        temp_output=$output"$level- [$title](#$anchor)\n"
        counter=1
        while true; do
            nlines="$(echo -e $temp_output | wc -l)"
            duplines="$(echo -e $temp_output | sort | uniq | wc -l)"
            if [ $nlines = $duplines ]; then
                break
            fi
            temp_output=$output"$level- [$title](#$anchor-$counter)\n"
            counter=$(($counter+1))
        done

        output="$temp_output"

    done <<< "$(grep -E '^#{2,10} ' "$1" | tr -d '\r')"

    echo "$output"

}

insert() {

    local toc_text="$2"
    local appname='doctoc.sh'
    # inspired in doctoc lines
    local start_toc='<!-- START '$appname' generated TOC please keep comment here to allow auto update -->'
    local info_toc='<!-- DO NOT EDIT THIS SECTION, INSTEAD RE-RUN '$appname' TO UPDATE -->'
    local end_toc='<!-- END '$appname' generated TOC please keep comment here to allow auto update -->'

    toc_block="$start_toc\n$info_toc\n**Table of Contents**\n\n$toc_text\n$end_toc"

    # temporary replace of '/' (confused with separator of substitutions) and '&' (confused with match regex symbol) to run the special sed command
    toc_block="$(echo "$toc_block" | sed 's,&,id9992384923423gzz,g')"
    toc_block="$(echo "$toc_block" | sed 's,/,id8239230090230gzz,g')"

    # search multiline toc block -> https://stackoverflow.com/questions/2686147/how-to-find-patterns-across-multiple-lines-using-grep/2686705
    # grep color for debugging -> https://superuser.com/questions/914856/grep-display-all-output-but-highlight-search-matches
    if grep --color=always -Pzl "(?s)$start_toc.*\n.*$end_toc" $1 &>/dev/null; then
        echo -e "\n  Updated content of $appname block in $1 succesfully\n"
        # src https://askubuntu.com/questions/533221/how-do-i-replace-multiple-lines-with-single-word-in-fileinplace-replace
        sed -i ":a;N;\$!ba;s/$start_toc.*$end_toc/$toc_block/g" $1
    else
        echo -e "\n  Created $appname block in $1 succesfully\n"
        sed -i '.bcp' -e 1i"$toc_block" "$1"
    fi

    # undo symbol replacements
    # sed -i 's,id9992384923423gzz,&,g' $1
    # sed -i 's,id8234923000230gzz,/,g' $1

}

main() {

    check "$1"
    toc_text=$(toc "$1")
    insert "$1" "$toc_text"

}

[[ "$0" == "$BASH_SOURCE" ]] && main "$@"
