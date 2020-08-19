BEGIN {
	a = 0
	tsi = 0
	ar="[]"
	tr[ar"byte"]="bytes"
	tr["abstract.Point"]="bytes"
	tr["StateAction"]="int"
	tr["byzcoin.InstanceID"]="bytes"
	tr["Nonce"]="bytes"
	print "syntax = \"proto2\";"
}


# a is the state

# state 0: look for proto start
a == 0 && /PROTOSTART/{ a = 1; next }

# state 1: in a protostart block
#   store types for later replacements
a == 1 && /^\/\/ type/ { split($0, t, /:/); tsi++; ts[tsi, 1] = t[2]; ts[tsi, 2] = t[3]; next}
#   copy all other comment lines at the beginning
a == 1 && /^\/\// { sub( "^// *", "" ); print; next }
#   go on if non-comment line
a == 1 { a = 2; print; next }

# state 2: looking for a type structure, start it.
a == 2 && /^type.*struct/ {
	cap=toupper(substr($2,1,1)) substr($2,2)
	gsub(/[ \t]+$/, "", cap)
	mn[$2]=cap
	print "message " mn[$2] " {";
	a = 3;
	i = 1;
	next
}
a == 2 { print; next }

# state 3: processing fields of the struct

#   detect end of struct -> state 2
a == 3 && /^\}/ { print; a = 2; next }
#   detect "// optional" tag in Go -> state 4
a == 3 && / *\/\/ optional/ { a = 4; next }
#   ignore blank lines
a == 3 && /^[[:blank:]]*$/ { next }
#   ignore hidden fields
a == 3 && /^[[:blank:]]*[[:lower:]]/ { next }
#   copy comments through
a == 3 && /[[:blank:]]*\/\// { sub( "[[:blank:]]*//[[:blank:]]*", "" ); print "  //", $0; next }
a == 3 && /\*/ { sub( "\\*", "", $2 )
					print_field("optional", $2, $1, i)
					i = i + 1
					next
				}
a == 3 && /.*`protobuf:"opt"`/ {
					print_field("optional", $2, $1, i)
					i = i + 1
					next
				}
a == 3 {
			print_field("required", $2, $1, i)
			i = i + 1
			next
		}

# state 4: manual optional
a == 4 { sub(/\/\/.*/, "", $2)
			print_field("optional", $2, $1, i)
			i = i + 1
			a = 3
			next
		}

function print_field( optional, typ, name, ind ){
	packed = ""

	if ( typ in tr )
		typ = tr[typ]
	if ( name ~ /bytes/ ){
		optional = "repeated"
	}
	for (c = 1; c <= tsi; c++){
		sub("^\\[\\]" ts[c, 1], "[]" ts[c, 2], typ)
		sub("^\\[\\]\\*" ts[c, 1], "[]*" ts[c, 2], typ)
		sub("^" ts[c, 1], ts[c, 2], typ)
	}
	if ( typ ~ /map/ ){
		optional = ""
	}

	if ( typ ~ /^\[\]/ ){
		optional = "repeated"
		sub(/^\[\]/, "", typ)
		if (typ ~ /^(int|u?int32|u?int64|bool)$/) {
			packed = " [packed=true]"
		}
	}
	sub(/^\[.*\]byte$/, "bytes", typ)
	sub(/^time.Duration$/, "sint64", typ)
	sub(/^kyber.Point$/, "bytes", typ)
	sub(/^kyber.Scalar$/, "bytes", typ)
	sub(/^float64$/, "double", typ)
	sub(/^float32$/, "float", typ)
	sub(/^int32$/, "sint32", typ)
	sub(/^int64$/, "sint64", typ)
	sub(/^int$/, "sint32", typ)
	sub(/^\*/, "", typ)

	# If we remembered a message name like this type, then
	# use it.
	if ( mn[typ] ) {
		typ=mn[typ]
	}

	print sprintf("  %s %s %s = %d%s;", optional, typ, tolower(name), ind, packed )
}
