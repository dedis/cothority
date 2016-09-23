#!/usr/bin/env ruby
#
#
require 'optparse'
require 'optparse/time'

require_relative 'log'
require_relative 'scrapper'
require_relative 'processor'

$opts = {:packages => [],
         :since => Time.new(2005,03,12),
         :until => Time.now ,
         :folder => "snapshots"}

parser = OptionParser.new do |opts|
    opts.banner = "Debian snapshot scrapper"
    opts.on("-s","--since TIME","Scraps from given time ISO 8601 format (%Y-%m-%d)") do |t|
        $opts[:since] = Time.strptime(t,"%F")
    end

    opts.on("-u","--until TIME","Scraps until given time ISO 8601 format") do |t|
        $opts[:until] = Time.strptime(t,"%F")
    end
    opts.on("-v","--verbose","Verbosity to debug") do |v|
        $logger.level = Logger::DEBUG
    end

end

parser.parse!

# args read the arguments either from STDIN or the file and puts the list of
# packages into the :packages key in @@opts
def check_args
    if STDIN.tty? && ARGV.empty? 
        $logger.info "No packages list given. Will retrieve all packages (time consuming)."
        return
    end 
    reader = STDIN.tty? ? ARGV.shift : STDIN
    reader.each_line do |line|
        $opts[:packages] << line.strip 
    end
end

def main
    check_args
    $logger.info "Crawling range #{$opts[:since]} to #{$opts[:until]}"
    scrapper =  Scrapper.new $opts[:packages], $opts[:since], $opts[:until]
    links = scrapper.scrap
    formatter = Formatter.new $opts[:folder],$opts[:packages]
    formatter.format links 
end

def help
    puts 'This ruby script will parse the website http://snapshot.debian.org/.
It takes a list of packages it needs to crawl, a start date and a end date.
The output is a csv file which is organized as:
snapshot_time, pkgName, version'
end

main
