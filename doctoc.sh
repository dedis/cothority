#!/bin/bash
#
# Takes a mardown file in argument, checks if a table of content is already
# present, create a new one if there is none or update the current one.
#
# The script uses the $start_toc and $end_doc as delimiters for the table of
# content. It works only if there is one pair of opening/closing delimiters. If
# it finds a correct single pair, the content inside is updated.
#
# Attributions: Updated from https://gitlab.com/pedrolab/doctoc.sh/tree/master,
# which comes from
# https://gist.github.com/meleu/57867f4a01ede1bd730f14b2f018ae89.
#
# The list of invalid chars come from https://github.com/thlorenz/anchor-\
# markdown-header/blob/56f77a232ab1915106ad1746b99333bf83ee32a2/anchor-\
# markdown-header.js#L25
#

# Exits early in case of errors
set -e
set -u

INVALID_CHARS="'[]/?!:\`.,()*\";{}+=<>~$|#@&–—"

# For Mac user, we need to use 'gsed'
SED=sed

appname=`basename $0`
start_toc='<!-- START '$appname' generated TOC please keep comment here to allow auto update -->'
info_toc='<!-- DO NOT EDIT THIS SECTION, INSTEAD RE-RUN '$appname' TO UPDATE -->'
end_toc='<!-- END '$appname' generated TOC please keep comment here to allow auto update -->'

check() {
    # Check if argument found
    if [ -z "$1" ]; then
        echo "Error. No argument found. Put as argument a file.md"
        exit 1
    fi
    # Check if file found
    [[ ! -f "$1" ]] && echo "Error. File not found" && exit
    # Check if used from MacOS
    if [[ "${OSTYPE//[0-9.]/}" == "darwin" ]]; then
        if type gsed >/dev/null 2>&1; then
            SED=gsed
        else
            warn="WARNING: Detecting you are on mac but didn't find the 'gsed' "
            warn+="utility. Default 'sed' version of mac is not likely to work " 
            warn+="here. You can install 'gsed' with 'brew install gnu-sed'."
            echo $warn
        fi
    fi
}

# This function parses each line of the file given in input. Each line is first
# parsed by awk in order to get only the ones concerning a title (ie. starting
# with a '#') and not in a code block (ie. inside a block defined by '```').
# Then, in the while loop, we extract the "level": number of '#' minus one
# converted to spaces, the "title": the line without the '#', and then the
# "anchor": the title without invalid chars and uppercase and with spaces
# converted to dashes. In the case there is duplicated titles, a "-n" token is
# finally appended to the anchor, where "n" is a counter. Here is an example
# with two titles and the content of the corresponding variables:
#
# # Sample title!
# ## Sample title!
#
# This yields for each title:
#
# level=""
# title="Sample title!"
# anchor="sample-title"
# output="- [Sample title!](sample-title)"
#
# level=" "
# title="Sample title!"
# anchor="sample-title"
# output=" - [Sample title!](sample-title-2)"
toc() {

    local line
    local level
    local title
    local anchor
    local output

    while IFS='' read -r line || [[ -n "$line" ]]; do
        level="$(echo "$line" | $SED -E 's/^(#+).*/\1/; s/#/  /g; s/^  //')"
        title="$(echo "$line" | $SED -E 's/^#+ //')"
        anchor="$(echo "$title" | tr '[:upper:] ' '[:lower:]-' | tr -d "$INVALID_CHARS")"

        # Check that new lines introduced are not duplicated. If so, introduce a
        # number at the end copying doctoc behavior.
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

    done <<< "$(awk -v code='^```' ' BEGIN { in_code=0 }
    {
        if ($0 ~ code && in_code == 0) { in_code=1 }
        else if ($0 ~ code && in_code == 1) { in_code=0 }
        if ($0 ~ /^#{1,10}/ && in_code == 0) { print }
    }' $1 | tr -d '\r')"

    echo "$output"
}

# This function takes the file path and the table of content in argument. It
# adds the toc title, checks if there is already a toc present and either
# updates or inserts a toc.
insert() {

    local toc_text="$2"
    local appname='doctoc.sh'

    toc_block="$start_toc\n$info_toc\n**:book: Table of Contents**\n\n$toc_text\n$end_toc"

    # temporary replace '/' (confused with separator of substitutions) and '&'
    # (confused with match regex symbol) to run the special sed command
    toc_block="$(echo "$toc_block" | $SED 's,&,id9992384923423gzz,g')"
    toc_block="$(echo "$toc_block" | $SED 's,/,id8239230090230gzz,g')"

    # Check if there is a block that begins with $start_toc and ends with
    # $end_toc. We ensure there is a correct and single pair of opening/closing
    # delimiters.
    S=$(awk -v start="^$start_toc$" -v end="^$end_toc$" 'BEGIN { status=-1; start_c=0; end_c=0 }
        { if ($0 ~ start && start_c > 0) { 
            start_c+=1; status=10; exit
          }
          if ($0 ~ start) {
              start_c+=1
          }
          if (start_c == 1 && $0 ~ end) { 
              end_c+=1; status=0 
          }
          if (start_c == 0 && $0 ~ end) {
              status=11; exit
          }
          if (end_c > 1 ) {
              status=12; exit
          }
        } END { 
            if (start_c == 1 && end_c == 0) {
                status=13
            }
            print status }' $1)
    
    # If the status S is >=10, that means something went bad and we must abort.
    if [ $S -ge 10 ]; then 
        echo "got an error while checking the opening/closing tags:"

        case $S in
            10)      
                echo " - found more than 1 opening tag. Please fix that"
                ;;
            11)      
                echo " - found a closing tag before an opening one. Please fix that"
                ;;
            12)
                echo " - found more than 1 closing tag. Please fix that"
                ;; 
            13)
                echo " - found only an opening tag. Please fix that"
                ;; 
        esac
        exit 1
    fi

    if [ $S -eq 0 ]; then
        # ":a" creates label 'a'
        # "N" append the next line to the pattern space
        # "$!" if not the last line
        # "ba" branch (goto) label a
        # In short, this loops throught the entire file until the last line and
        # then performs the substitution.
        $SED -i ":a;N;\$!ba;s/$start_toc.*$end_toc/$toc_block/g" $1
        echo -e "\n  Updated content of $appname block in $1 succesfully\n"
    else
        $SED -i 1i"$toc_block" "$1"
        echo -e "\n  Created $appname block in $1 succesfully\n"
    fi

    # undo symbol replacements
    $SED -i 's,id9992384923423gzz,\&,g' $1
    $SED -i 's,id8239230090230gzz,/,g' $1

}

main() {
    check "$1"
    toc_text=$(toc "$1")
    insert "$1" "$toc_text"
}

[[ "$0" == "$BASH_SOURCE" ]] && main "$@"