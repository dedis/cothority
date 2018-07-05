BEGIN {
	a = 0
	ar="[]"
	tr[ar"byte"]="bytes"
	tr["abstract.Point"]="bytes"
	print "syntax = \"proto2\";"
}


# a is the state

# state 0: look for proto start
a == 0 && /PROTOSTART/{ a = 1; next }

# state 1: in a protostart block
a == 1 && /^\/\// { sub( "^// *", "" ); print; next }
a == 1 { a = 2; print; next }

# state 2: looking for a type structure, start it.
a == 2 && /^type.*struct/ { print "message", $2, "{"; a = 3; i = 1; next }
a == 2 { print; next }

# state 3: processing fields of the struct

#   detect end of struct -> state 2
a == 3 && /^\}/ { print; a = 2; next }
#   detect "// optional" tag in Go -> state 4
a == 3 && / *\/\/ optional/ { a = 4; next }
#   copy comments through
a == 3 && / *\/\// { sub( " *//\\s*", "" ); print "  //", $0; next }
a == 3 && /\*/ {    sub( "\\*", "", $2 )
					print_field("optional", $2, $1, i)
					i = i + 1
					next
				}
a == 3 { 	print_field("required", $2, $1, i)
			i = i + 1
			next
		}

# state 4: manual optional
a == 4 { print_field("optional", $2, $1, i)
			i = i + 1
			a = 3
			next
		}

function print_field( optional, typ, name, ind ){
	if ( typ in tr )
		typ = tr[typ]
	if ( name ~ /bytes/ ){
		optional = "repeated"
	}
	if ( typ ~ /^\[\]/ ){
		optional = "repeated"
		sub(/^\[\]/, "", typ)
	}
	sub(/^.*\./, "", typ)
	sub(/^Point$/, "bytes", typ)
	sub(/^Scalar$/, "bytes", typ)
	sub(/^ID$/, "bytes", typ)
	sub(/^SkipBlockID$/, "bytes", typ)
	sub(/^Role$/, "int", typ)
	sub(/^int32$/, "sint32", typ)
	sub(/^int64$/, "sint64", typ)
	sub(/^int$/, "sint32", typ)
	sub(/^\*/, "", typ)
	print sprintf("  %s %s %s = %d;", optional, typ, tolower(name), ind )
}
