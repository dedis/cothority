#!/usr/bin/env ruby
#
# Compute the costs given a snapshots.csv
#
require 'csv'
require 'time'
require_relative 'ruby_util'

AVERAGE_BUILD_TIME = 5.minutes

raise "No files given as argument " unless ARGV.size > 0
raise "File does not exists" unless File.exists? ARGV[0]

times = Hash.new{|h,k| h[k] = 0}
bw = Hash.new{|h,k| h[k] = 0}
builds = 0
CSV.foreach(ARGV[0],headers: true) do |row|
    t = Time.strptime(row[0],"%Y%m%d%H%M%S")
    times[t] += 1
    bw[t] += row[5].to_i
    builds += 1
end

## averages of how many builds per days,month or year 
total = times.inject(0) {|acc,(t,nb)| acc += nb }
min_date = times.keys.min
max_date = times.keys.max
## range of analyzed dates in seconds
range = max_date - min_date

avg_day = total / (range / 60 / 60 / 24)
avg_month = total / (range / 60 / 60 / 24 / 30)
avg_year = total / (range / 60 / 60 / 24 / 30 / 365)

puts "Total # of builds:\t#{builds}"
puts "Range in days:\t\t#{(range/60/60/24).to_i}" 
puts "Average # of builds:"
puts "\tPer Day:    \t#{avg_day}"
puts "\tPer Month:  \t#{avg_month}"
puts "\tPer Year:   \t#{avg_year}"

puts "Average building time:\t#{AVERAGE_BUILD_TIME} minutes"
puts "\tPer Day:    \t#{avg_day*AVERAGE_BUILD_TIME}"
puts "\tPer Month:  \t#{avg_month*AVERAGE_BUILD_TIME}"
puts "\tPer Year:   \t#{avg_year*AVERAGE_BUILD_TIME}"

total_bw = bw.inject(0) {|acc,(t,nb)| acc += nb }
bw_day = total_bw / (range / 60 / 60 / 24)
bw_month = total_bw / (range / 60 / 60 / 24 / 30)
bw_year = total_bw / (range / 60 / 60 / 24 / 30 / 365)
puts "Total bandwidth (B):    #{total_bw}"
puts "Average bandwidth:"
puts "\tPer Day:     \t#{bw_day}"
puts "\tPer Month:     \t#{bw_month}"
puts "\tPer Year:     \t#{bw_year}"
