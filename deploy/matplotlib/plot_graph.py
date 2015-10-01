# Plots the graph of one of the test-runs
# It takes the CSV-file as argument and shows the plot
# of the times used for each round

import matplotlib.pyplot as plt
import matplotlib.ticker
import csv
import sys
import math

def delete_poly(poly):
    poly.remove()

def delete_line(line):
    line[0].remove()
    if len(line) > 1:
        for i in range(1, 3):
           for l in line[i]:
              l.remove()

# Our CSVs have a space after the comma, so we need a new 'dialect', here
# called 'deploy'
csv.register_dialect('deploy', delimiter=',',doublequote=False,quotechar='',lineterminator='\n',escapechar='',
    quoting=csv.QUOTE_NONE,skipinitialspace=True)

x=[]
tmin=[]
tmax=[]
avg=[]
std=[]
tsys=[]
tusr=[]
xmin = -1
xmax = 0
ymin = -1
ymax = 0

def readCSV(name):
    global x, tmin, tmax, avg, std, tsys, tusr, xmin, xmax, ymin, ymax
    x=[]
    tmin=[]
    tmax=[]
    avg=[]
    std=[]
    tsys=[]
    tusr=[]
    # Read in all lines of the CSV and store in the arrays
    with open(name) as csvfile:
        reader = csv.DictReader(csvfile, dialect='deploy')
        for row in reader:
            x.append(float(row['hosts']))
            tmin.append(float(row['min']))
            tmax.append(float(row['max']))
            avg.append(float(row['avg']))
            std.append(float(row['stddev']))
            tsys.append(float(row['systime']))
            tusr.append(float(row['usertime']))
    # I suppose that x is > 0 anyway, so I can test on -1
    # and max will always be >= 0
    if xmin == -1:
        # Suppose it's the start, so also init ymin
        xmin = min(x)
        ymin = min(avg)
    else:
        xmin = min(xmin, min(x))
        ymin = min(ymin, min(avg))
    xmax = max(xmax, max(x))
    ymax = max(ymax, max(tmax))

if len(sys.argv) == 1:
    print("Error: Please give a .csv-file as argument\n")
    exit(1)

readCSV(sys.argv[1])

# Plot min, max and the avg with the standard-deviation
l1 = plt.fill_between(x, tmin, tmax, facecolor='honeydew')
# Make an empty plot for the legend
#plt.plot([], [], color='honeydew', linewidth=10, label="MinMax")
l2 = plt.errorbar(x, avg, std, None, label='Cothority', linestyle='-', marker='^', color='green')
l3 = None
l4 = None

if len(sys.argv) > 2:
    readCSV(sys.argv[2])

    # Plot min, max and the avg with the standard-deviation
    l3 = plt.fill_between(x, tmin, tmax, facecolor='#FCDFFF')
    # Make an empty plot for the legend
    #plt.plot([], [], color='red', linewidth=10, label="Shamir sign")
    l4 = plt.errorbar(x, avg, std, None, label='JVSS', linestyle='-', marker='^', ecolor='red')


# Add labels to the axis
plt.ylabel('seconds')
plt.xlabel('hosts')
plt.legend()

# Set the limits of the axes
plt.ylim(ymin, ymax)
plt.xlim(xmin, xmax * 1.2)
# X-axis is logarithmic, but we don't want to show 2^1, 2^2, ... but
# 2, 4, 8, ...
plt.xscale(u'log', basex=2)
plt.yscale('log', basey=2)
ax = plt.axes()
ax.xaxis.set_major_formatter(matplotlib.ticker.ScalarFormatter())
ax.yaxis.set_major_formatter(matplotlib.ticker.ScalarFormatter())
#plt.show()

# Finally save the graph
plt.savefig(sys.argv[1] + ".round.png")

# So we can still use the labels and axis we created
delete_poly(l1)
delete_line(l2)
plt.ylim(0, 1)
plt.plot(x, tsys, '-', label="System time")
plt.plot(x, tusr, '-', label="User time")
plt.legend(['System time', 'User time'])
plt.savefig(sys.argv[1] + ".time.png")
