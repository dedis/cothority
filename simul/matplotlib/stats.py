# Reads the stats of a given run and returns easy-to-use data with mplot

import csv
import unittest
import numpy as np

# Our CSVs have a space after the comma, so we need a new 'dialect', here
# called 'deploy'
csv.register_dialect('deploy', delimiter=',', doublequote=False, quotechar='',
                     lineterminator='\n', escapechar='',
                     quoting=csv.QUOTE_NONE, skipinitialspace=True)


# CSVStats holds all data from one run
class CSVStats:
    x = []

    # reads in a cvs and fills up the corresponding arrays
    # also fills in xmin, xmax, ymin and ymax which are
    # valid over multiple calls to readCVS!
    # If you want to start a new set, put xmin = -1
    def __init__(self, file, x_id=0):
        self.x = np.array([])
        self.columns = {}
        self.file = file
        # Read in all lines of the CSV and store in the arrays
        with open(file) as csvfile:
            reader = csv.DictReader(csvfile, dialect='deploy')
            for row in reader:
                for column, value in row.iteritems():
                    if not column in self.columns:
                        self.columns[column] = np.array([])
                    if value == None or type(value) is list:
                        print "Invalid value in file " + file + ":" + str(
                            type(value))
                    else:
                        self.columns[column] = np.append(self.columns[column],float(value))

        if type(x_id) == str:
            if x_id in self.columns.keys():
                self.x = self.columns[x_id]
            else:
                print "Didn't find key " + x_id + " in file " + self.file
        else:
            col = sorted(self.columns.keys())[x_id]
            self.x = self.columns[col]

    # Returns a Values-object with the requested column.
    # Updates the self.(x|y)(min|max)
    def get_values(self, column):
        values = Values(self.x, column, self.columns)
        return values

    # Returns a Values-object with the requested column, but only returns
    # those elements where the 'filter_column' has the 'filter_value'.
    # Updates the self.(x|y)(min|max)
    def get_values_filtered(self, column, filter_column, filter_value):
        filter = np.argwhere(self.columns[filter_column] == filter_value)
        values = Values(self.x, column, self.columns, filter=filter)
        return values

    # get_min_max runs over all values and returns minimum and maximum values
    # for both x- and y-coordinates
    @staticmethod
    def get_min_max(*vals):
        values_y = []
        values_x = []
        for v in vals:
            values_y += [v.ymin, v.ymax]
            values_x += v.x
        return (min(values_x), max(values_x), min(values_y), max(values_y))

    # for old data, that don't have yet a 'depth'-field in the csv.
    def get_old_depth(self):
        old_hosts = self.columns['hosts']
        old_bf = self.columns['bf']
        old_depth = []
        for x in range(0, len(old_hosts)):
            bf = old_bf[x]
            hosts = old_hosts[x]
            old_depth.append(0)
            for depth in range(0, 10):
                h = (1 - (bf ** depth)) / (1 - bf)
                if h <= hosts:
                    old_depth[x] = depth
        return old_depth

    # adjust that column-avg,min,max with that value
    def column_add(self, column, dx):
        for i in range(0, len(self.x)):
            for t in ['avg', 'min', 'max']:
                self.columns[column + "_" +t][i] += dx

    # adjust that column-avg,min,max by multiplying with that value
    def column_mul(self, column, dx):
        for i in range(0, len(self.x)):
            for t in ['avg', 'min', 'max']:
                self.columns[column + "_" +t][i] *= dx

    # Cut that index out of all columns
    def delete_index(self, i):
        for c in self.columns.keys():
            if len(self.x) == len(self.columns[c]):
                del self.columns[c][i]


# Value holds the min / max / avg / dev for a single named value
class Values:
    def __init__(self, x, column, columns, filter=None):
        self.name = column
        self.columns = columns
        self.x = np.array(x)

        # Set min, max, avg, dev-values from csv-file
        self.min = np.array(self.has_column(column + "_min"))
        self.max = np.array(self.has_column(column + "_max"))
        self.avg = np.array(self.has_column(column + "_avg"))
        self.dev = np.array(self.has_column(column + "_dev"))
        if filter is not None:
            self.x = np.transpose(np.choose(filter, self.x))
            self.min = np.transpose(np.choose(filter, self.min))[0]
            self.max = np.transpose(np.choose(filter, self.max))[0]
            self.avg = np.transpose(np.choose(filter, self.avg))[0]
            self.dev = np.transpose(np.choose(filter, self.dev))[0]

        self.ymin = min(self.min)
        self.ymax = max(self.max)

    # Returns the column if it exists, else a single [1]
    def has_column(self, column):
        if column in self.columns:
            return self.columns[column]
        else:
            return [1]


# At least some tests for this module
# Lost "test.csv"-file :(
# class TestStringMethods(unittest.TestCase):
#     def test_load(self):
#         stats = CSVStats("test.csv")
#         self.assertEqual(stats.x, [1, 2, 4, 8], "x-values not correct")
#         stats = CSVStats("test.csv", 0)
#         self.assertEqual(stats.x, [1, 2, 4, 8], "x-values not correct")
#         stats = CSVStats("test.csv", 'Hosts')
#         self.assertEqual(stats.x, [1, 2, 4, 8], "x-values not correct")
#         stats = CSVStats("test.csv", 'round_min')
#         self.assertEqual(stats.x, [2, 3, 4, 5], "x-values not correct")
#
#     def test_min(self):
#         stats = CSVStats("test.csv")
#         stats.update_values('round')
#         self.assertEqual(stats.min, [2, 3, 4, 5],
#                          "minimum of round not correct")
#         self.assertEqual(stats.max, [6, 7, 8, 9],
#                          "maximum of round not correct")
#         self.assertEqual(stats.avg, [3, 4, 5, 6],
#                          "average of round not correct")
#         self.assertEqual(stats.dev, [1, 1, 1, 1],
#                          "deviation of round not correct")
#         self.assertEqual(stats.xmin, 1)
#         self.assertEqual(stats.xmax, 8)
#         self.assertEqual(stats.ymin, 2)
#         self.assertEqual(stats.ymax, 9)

class TestFilter(unittest.TestCase):
    def setUp(self):
        with open("/tmp/test.csv", "w") as tmpfile:
            tmpfile.write(
                """hosts,BF,time_min,time_max,time_avg,time_dev
16, 2, 1.0, 3.0, 2.0, 0.
16, 4, 2.0, 6.0, 4.0, 0.
32, 2, 3.0, 9.0, 6.0, 0.
32, 4, 4.0, 12.0, 8.0, 0.""")

    def test_filter(self):
        stats = CSVStats("/tmp/test.csv", "hosts")
        print("Stats:", stats.x)
        self.assertEqual(stats.x.tolist(), [16, 16, 32, 32], "x-values not correct")
        filtered = stats.get_values_filtered('time', 'BF', 2.0)
        print("Filtered:", filtered.min.tolist())
        self.assertEqual(filtered.min.tolist(), [1.0, 3.0], "not good filter")

# Run the tests if we're run directly
if __name__ == '__main__':
    unittest.main()
