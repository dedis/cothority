module RubyUtil

    ## remove the module name 
    def self.demodulize(path)
        path = path.to_s
        if i = path.rindex('::')
            path[(i+2)..-1]
        else
            path
        end
    end    

    def self.max coll
        coll.inject(0) { |col,value| value > col ? value : col }
    end

    ## Find the common subset of all sets / array given
    def self.commom_subset *lists
        return [] if lists.size == 0
        return lists.first if lists.size == 1
        sub = lists.shift
        lists.each do |list|
            sub = sub & list
        end
    end

    # Upper limit on number of elements
    # to when to partition a task
    # collection must respond to slice
    CHUNK_SIZE = 100
    def self.partition_by_size collection, chunk_size = CHUNK_SIZE
        return collection unless block_given?
        # No need for partitioning
        if collection.size <= chunk_size
            yield collection, 0
            return
        end

        counter = collection.size / chunk_size
        rest = collection.size % chunk_size
        # yield for each "slice"
        counter.times do |n|
            low = n * chunk_size
            sub = collection.slice(low,chunk_size) 
            yield sub,n
        end
        unless rest == 0
            # yield for the rest
            sub = collection.slice( counter*chunk_size, (counter*chunk_size) + rest)
            yield sub,counter
        end
    end

    ## how many slice do 
    #
    def self.slice collection, slice_number 
        return collection unless block_given?

        size_chunk = collection.size / slice_number
        size_rest = collection.size % slice_number
        # yield for each "slice"
        (slice_number-1).times do |n|
            low = n * size_chunk
            sub = collection.slice(low,size_chunk)
            yield sub,n
        end
        low = (slice_number-1) * size_chunk
        high = low + size_chunk + size_rest
        sub = collection.slice(low,high)
        yield sub,slice_number
    end


    module UnitsTest
        require 'test/unit'
        class TestRubyUtil < Test::Unit::TestCase

#            def test_slice
#                a = (0...100).to_a
#                times = 0
#                a.each_slice(4).each_with_index do |col,i|
#                    puts "(#{i} : #{col.size}"
#                    assert_equal(i,times)
#                    assert(times < 4)
#                    times += i
#                end
        
#                a = (0...120).to_a
#                times = 0
#                RubyUtil::slice a,4 do |col,i|
#                    assert_equal(i,times)
#                    assert(times < 5)
#                    assert(col.size == 20) if i == 4
#                    times += 1
#                end

#            end
            ## SHOULD NOT WORK !
            #def test_partition_slice
                #a = (0...100).to_a 
                #times = 0
                #RubyUtil::partition_by_size a,25 do |col,i|
                    #assert_equal(25,col.size)
                    #assert(times < 4)
                    #assert(times == i)
                    #times += 1
                #end

                #a = (0...120).to_a
                #times = 0
                #RubyUtil::partition_by_size a,25 do |col,i|
                    #assert_equal(times,i)
                    #assert(times < 5)
                    #assert(col.size == 20) if i == 4
                    #times += 1
                #end
            #end
        end
    end

    ## Can symbolize keys of a Hash, or every elements of an Array
    # Can apply a preprocessing step on the element with :send opts,
    # it will call the method on each element
    def self.symbolize coll,opts = {}
        if coll.is_a?(Hash)
            return coll.inject({}) do |new,(k,v)|
                nk = k
                nk = k.send(opts[:send]) if opts[:send]
                nk = nk.to_sym
                if v.is_a?(Hash)
                    new[nk] = RubyUtil::symbolize(v)
                else
                    new[nk] = v
                    new[nk] = v.to_sym if opts[:values]
                end
                new
            end 
        elsif coll.is_a?(Array)
            return coll.map { |c| opts[:send] ? c.send(opts[:send]).to_sym : c.to_sym }
        elsif coll.is_a?(String)
            return coll.to_sym
        end
    end
    def self.arrayize value
        if value.is_a?(Array)
            value
        else
            [ value ]
        end
    end
    def self.escape value
        esc = lambda {|x| x.gsub(/\\|'/) { |c| "\\#{c}" } }
        if value.is_a?(String)
            esc.call(value)
        elsif value.is_a?(Array)
            value.map { |v| self.escape(v) }
        else
            value
        end
    end
    def self.quotify list
        list.map { |l| "'#{self.escape(l)}'"}
    end
    def self.sqlize list,opts = {}
        str = opts[:no_quote] ? list : RubyUtil::quotify(list)
        opts[:no_parenthesis] ? str.join(',') : "(" + str.join(',') + ")"
    end

    def self.require_folder folder
        Dir["#{folder}/*.rb"].each { |f| require_relative "#{f}" }
    end


end
# must be outside of the module
# if not, different namespace will be created
class Fixnum
    MIN_IN_HOURS = 60
    HOURS_IN_DAY = 24
    ## Approximatively
    DAYS_IN_MONTH = 30
    def months
        days * DAYS_IN_MONTH
    end
    alias :month :months

    def days
        hours * HOURS_IN_DAY
    end
    alias :day :days

    def hours
        minutes * MIN_IN_HOURS
    end
    alias :hour :hours

    def minutes
        self
    end
    alias :minute :minutes
end

require 'singleton'
class SignalHandler
    include Singleton

    @@count = 0
    MAX_TRY = 5
    def initialize 
        @breaker = false
        @blocks = []
    end

    def self.ensure_block &block
        @blocks = [] unless @blocks
        @blocks << block
    end

    def self.enable
        trap("INT") do
            @breaker = true
            yield if block_given?
            exit if @debug
            exit if @@count > MAX_TRY
            @@count += 1
        end
    end

    def self.debug
        @debug = true
    end

    def self.check
        if @breaker
            yield if block_given?
            @blocks.each { |b| b.call }
            exit
        end
    end
end
