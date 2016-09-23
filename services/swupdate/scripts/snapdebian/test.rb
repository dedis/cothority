require 'debian_control_parser'

data=<<EOF
Foo: bar
Answer-to-Everything: 42

Ding: dong
Zingo: zongo
Multi:
 several lines
 in a paragraph
 are of course allowd
Final: field
EOF

parser = DebianControlParser.new(data)
parser.paragraphs do |paragraph|
  paragraph.fields do |name,value|
    puts "Name=#{name} / Value=#{value}"
  end
end

