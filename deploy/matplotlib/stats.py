# Reads the stats of a given run and returns easy-to-use data

import csv
import unittest


# Our CSVs have a space after the comma, so we need a new 'dialect', here
# called 'deploy'
csv.register_dialect('deploy', delimiter=',', doublequote=False, quotechar='', lineterminator='\n', escapechar='',
                     quoting=csv.QUOTE_NONE, skipinitialspace=True)


class CSVStats:
    x = []

    # reads in a cvs and fills up the corresponding arrays
    # also fills in xmin, xmax, ymin and ymax which are
    # valid over multiple calls to readCVS!
    # If you want to start a new set, put xmin = -1
    def __init__(self, file, x_id=0):
        self.x = []
        self.columns = {}
        # Read in all lines of the CSV and store in the arrays
        with open(file) as csvfile:
            reader = csv.DictReader(csvfile, dialect='deploy')
            for row in reader:
                for column, value in row.iteritems():
                    if not column in self.columns:
                        self.columns[column] = []
                    self.columns[column] += [float(value)]

        if type(x_id) == str:
            self.x = self.columns[x_id]
        else:
            col = sorted(self.columns.keys())[x_id]
            self.x = self.columns[col]

    # Returns a Values-object with the requested column.
    # Updates the self.(x|y)(min|max)
    def get_values(self, column):
        values = Values(self.x, column, self.columns)
        return values


    @staticmethod
    def get_min_max(*vals):
        values_y = []
        values_x = []
        for v in vals:
            values_y += [v.ymin, v.ymax]
            values_x += v.x
        return (min(values_x), max(values_x),min(values_y), max(values_y))

    def add(self, stats, col1, col2):
        sum = deepcopy(self)

# Value holds the min / max / avg / dev for a single named value
class Values:
    def __init__(self, x, column, columns):
        self.name = column
        self.columns = columns
        self.x = x

        # Set min, max, avg, dev-values from csv-file
        self.min = self.has_column(column + "_min")
        self.max = self.has_column(column + "_max")
        self.avg = self.has_column(column + "_avg")
        self.dev = self.has_column(column + "_dev")
        self.ymin = min(self.min)
        self.ymax = max(self.max)

    def has_column(self, column):
        if column in self.columns:
            return self.columns[column]
        else:
	    return [1]

class TestStringMethods(unittest.TestCase):
    def test_load(self):
        stats = CSVStats("test.csv")
        self.assertEqual(stats.x, [1, 2, 4, 8], "x-values not correct")
        stats = CSVStats("test.csv", 0)
        self.assertEqual(stats.x, [1, 2, 4, 8], "x-values not correct")
        stats = CSVStats("test.csv", 'Hosts')
        self.assertEqual(stats.x, [1, 2, 4, 8], "x-values not correct")
        stats = CSVStats("test.csv", 'round_min')
        self.assertEqual(stats.x, [2, 3, 4, 5], "x-values not correct")

    def test_min(self):
        stats = CSVStats("test.csv")
        stats.update_values('round')
        self.assertEqual(stats.min, [2, 3, 4, 5], "minimum of round not correct")
        self.assertEqual(stats.max, [6, 7, 8, 9], "maximum of round not correct")
        self.assertEqual(stats.avg, [3, 4, 5, 6], "average of round not correct")
        self.assertEqual(stats.dev, [1, 1, 1, 1], "deviation of round not correct")
        self.assertEqual(stats.xmin, 1)
        self.assertEqual(stats.xmax, 8)
        self.assertEqual(stats.ymin, 2)
        self.assertEqual(stats.ymax, 9)


if __name__ == '__main__':
    unittest.main()
