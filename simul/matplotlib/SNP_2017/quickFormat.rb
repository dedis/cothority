#!/usr/bin/env ruby
#
raise "[-] Give a file in csv format !" unless ARGV[0]
raise "[-] Give columns to print 'c1,c2,c3'" unless ARGV[1]

file = ARGV[0]
columns = ARGV[1].split(",").map(&:strip)

fd = File.open(file,"r")
header = fd.readline
columnsIdx = header.split(",").each_with_index.map{ |c,i| i if columns.include? c }.compact

puts columns.join(",")
fd.each_line.each_with_index do |l,i|
    puts l.split(",").values_at(*columnsIdx).join(",")
end

fd.close
