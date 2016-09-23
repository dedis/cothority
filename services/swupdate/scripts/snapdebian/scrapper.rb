require_relative 'log'
require 'mechanize'

## URLs will be taken such as
# SNAPSHOT_URL / <timestamp> / PATH / {BINARY_PATH,SOURCE_PATH}
SNAPSHOT_URL = "http://snapshot.debian.org/archive/debian/"
PATH = "dists/testing/main/"
#PATH = "dists/jessie/updates/main/"
BINARY_PATH = "binary-amd64/Packages.gz"
SOURCE_PATH = "source/Sources.gz"

class Scrapper


    def initialize packages,from,to
        @packages = packages
        @from = from
        @to = to
        @agent = Mechanize.new
        @folder = $opts[:folder] || "snapshots"
    end

    def scrap 
        agent = Mechanize.new
        flinks = links 
        $logger.info "Found #{flinks.size} snapshots falling between given dates" 
        # create the full links by appending the path for binary + source
        # Hash[snapshotTime] = { :binary => *link*, :source => *link* }
        flinks.inject(Hash.new{ |h,k| h[k] = {}}) do |h,link|
            h[link.text][:binary] = URI.join(@agent.page.uri.merge(link.uri),PATH,BINARY_PATH)
            h[link.text][:source] = URI.join(@agent.page.uri.merge(link.uri),PATH,SOURCE_PATH)
            h
        end
    end

    private 

    # take the main page and find the range links for year + months 
    # + days + hours
    def links
        from_rounded = round_time @from
        to_rounded = round_time @to
        first_links = @agent.get(SNAPSHOT_URL).links_with(href: /.\/?year=/) 
        # filter by year-month from < "year-month" > to
        first_links.select! do |links| 
            links.href =~ /.\/?year=([0-9]{4})&month=([0-9]{1,2})/
            year,month = $1,$2
            t = Time.strptime("#{year}-#{month}","%Y-%m")
            v1 = from_rounded <= t 
            v2 = t <= to_rounded
            #$logger.debug "#{from_rounded} < #{t} => #{v1} || < #{to_rounded} => #{v2}"
            v1 && v2
        end
        seconds = first_links.inject([]) do |acc,link|
            page = link.click
            ## select all valid time links
            page.links.each do |timeLink|
                begin
                    exact = Time.parse timeLink.text
                    acc << timeLink if @from <= exact && exact <= @to
                    #puts "#{exact} => #{@from <= exact && exact <= @to}"
                rescue
                    next 
                end
            end
            acc
        end
        seconds
    end

    def round_time time, year=true, month= true
        format = "" 
        toParse = ""
        if year
            format += "%Y" 
            toParse += time.year.to_s
        end
        if month
            format += "-%m"
            toParse += "-#{time.month.to_s}"
        end
        Time.strptime(toParse,format)
    end
end
