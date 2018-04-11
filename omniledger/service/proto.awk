BEGIN {
	a = 0
	ar="[]"
	tr["[]byte"]="bytes"
	tr["abstract.Point"]="bytes"
	tr["Version"]="sint32"
	print "syntax = \"proto2\";"
}

a == 0 && /PROTOSTART/{ a = 1; next }

a == 1 && /^\/\// { sub( "^\/\/ *", "" ); print; next }
a == 1 { a = 2; print; next }

a == 2 && /^type.*struct/ { print "message", $2, "{"; a = 3; i = 1; next }
a == 2 { print; next }

a == 3 && /^\}/ { print; a = 2; next }
a == 3 && / *\/\// { sub( " *\/\/\s*", "" ); print "  //", $0; next }
a == 3 && /\*/ {    sub( "\\*", "", $2 )
					print_field("optional", $2, $1, i)
					i = i + 1
					next
				}
a == 3 { 	print_field("required", $2, $1, i)
			i = i + 1
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
	sub(/^\[\]byte$/, "bytes", typ)
	sub(/^\*/, "", typ)
	print sprintf("  %s %s %s = %d;", optional, typ, tolower(name), ind )
}
