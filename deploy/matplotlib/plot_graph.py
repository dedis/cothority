# Plots the graph of one of the test-runs
# It takes the CSV-file as argument and shows the plot
# of the times used for each round

import matplotlib.pyplot as plt
import matplotlib.ticker
import csv
import sys

if len(sys.argv) == 1:
    print("Error: Please give a .csv-file as argument\n")
    exit(1)

x=[]
min=[]
max=[]
avg=[]
std=[]

# Our CSVs have a space after the comma, so we need a new 'dialect', here
# called 'deploy'

csv.register_dialect('deploy', delimiter=',',doublequote=False,quotechar='',lineterminator='\n',escapechar='',
    quoting=csv.QUOTE_NONE,skipinitialspace=True)

# Read in all lines of the CSV and store in the arrays
with open(sys.argv[1]) as csvfile:
    reader = csv.DictReader(csvfile, dialect='deploy')
    for row in reader:
        x.append(float(row['hosts']))
        min.append(float(row['min']))
        max.append(float(row['max']))
        avg.append(float(row['avg']))
        std.append(float(row['stddev']))

# Plot min, max and the avg with the standard-deviation
plt.plot(x, min, '-', label='min')
plt.errorbar(x, avg, std, None, label='avg', linestyle='-', marker='^')
plt.plot(x, max, '-', label='max')

# Add labels to the axis
plt.ylabel('seconds')
plt.xlabel('hosts')
plt.legend()

# Set the limits of the axes
plt.ylim(0, 5)
# X-axis is logarithmic, but we don't want to show 2^1, 2^2, ... but
# 2, 4, 8, ...
plt.xscale(u'log', basex=2)
ax = plt.axes()
ax.xaxis.set_major_formatter(matplotlib.ticker.ScalarFormatter())

# Finally show the graph
plt.show()
