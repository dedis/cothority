# Plots the graph of one of the test-runs
# It takes the CSV-file as argument and shows the plot
# of the times used for each round

import matplotlib.pyplot as plt
import matplotlib.ticker
import csv
import sys
import math

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

# If we want to remove a poly
def delete_poly(poly):
    poly.remove()

# For removing a line
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

# reads in a cvs and fills up the corresponding arrays
# also fills in xmin, xmax, ymin and ymax which are
# valid over multiple calls to readCVS!
# If you want to start a new set, put xmin = -1
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

# Adds a fill_between and the corresponding 'empty' plot to show up in
# the legend
def plotFilledLegend(x, y1, y2, label, color):
    plt.fill_between(x, y1, y2, facecolor=color)
    plt.plot([], [], '-', label=label, color=color, linewidth=10)

# Takes one x and y1, y2 to stack y2 on top of y1. Does all the
# calculation necessary to sum up everything
def plotStacked(x, y1, y2, label1, label2, color1, color2, ymin = None):
    if ymin == None:
        ymin = min(min(y1), min(y2))
    ymins = [ymin] * len(x)
    ysum = [sum(t) for t in zip(y1, y2)]
    plotFilledLegend(x, y1, ysum, label2, color2)
    plotFilledLegend(x, ymins, y1, label1, color1)

# Puts the most used arguments for starting a plot with
# LogLog by default.
def plotPrepareLogLog(logx = True, logy = True):
    plt.clf()
    plt.ylabel('seconds')
    plt.xlabel('hosts')
    if logx:
        plt.xscale(u'log', basex=2)
    if logy:
        plt.yscale(u'log', basey=2)
    ax = plt.axes()
    ax.xaxis.set_major_formatter(matplotlib.ticker.ScalarFormatter())
    #ax.yaxis.set_major_formatter(matplotlib.ticker.ScalarFormatter())

# Ends the plot and takes an extension for saving the png. If
# show_fig is True, it will show the window instead.
def plotEnd(name, extension):
    global show_fig
    if show_fig:
        plt.show()
    else:
        save = name + "." + extension + ".png"
        print "Saving to", save
        plt.savefig(save)

# This one takes two csv-files which represent a Cothority and a JVSS
# run, stacking the user and system-time one upon the other.
def CoJVTime(cothority, jvss):
    plotPrepareLogLog();
    xmin = -1
    readCSV(jvss)
    plotStacked(x, tsys, tusr, "JVSS system time", "JVSS user time",
        color2_light, color2_dark )
    mm = [min(tsys), max(tusr)]

    readCSV(cothority)
    plotStacked(x, tsys, tusr, "Cothority system time", "Cothority user time",
        color1_light, color1_dark, min(mm) )
    mm = [min(mm[0], min(tsys)), max(mm[1], max(tusr))]

    plt.ylim(min(tsys), mm[1])
    plt.legend()
    plotEnd(cothority, "sys")

# Plots a Cothority and a JVSS run with regard to their averages. Supposes that
# the last two values from JVSS are off-grid and writes them with arrows
# directly on the plot
def plotAvg(cothority, jvss):
    plotPrepareLogLog()

    xmin = -1
    readCSV(jvss)
    plt.plot(x, avg, label='JVSS', linestyle='-', marker='^', color=color2_dark)
    plotFilledLegend(x, tmin, tmax, "min-max", color2_light)
    plt.annotate("{:.2f}".format(avg[-2]), xy=(x[-2], 4), xytext=(x[-2], 1.5),
                arrowprops=dict(facecolor='red', shrink=0.05) )
    plt.annotate("{:.2f}".format(avg[-1]), xy=(x[-1], 4), xytext=(x[-1], 2),
                arrowprops=dict(facecolor='red', shrink=0.05) )

    readCSV(cothority)
    plt.plot(x, avg, label='Cothority', linestyle='-', marker='o', color=color1_dark)
    plotFilledLegend(x, tmin, tmax, "min-max", color1_light)

    # Make horizontal lines and add arrows for JVSS
    plt.axes().yaxis.grid(color='gray', linestyle='dashed')
    plt.ylim(ymin, 4)
    plt.xlim(xmin, xmax * 1.2)

    plt.legend(loc = u'lower right')
    plotEnd(cothority, "avg")

# Colors for the Cothority
color1_light = 'lightgreen'
color1_dark = 'green'
# Colors for the JVSS
color2_light = '#FCDFFF'
color2_dark = 'red'
show_fig = False

if len(sys.argv) != 4:
    print("Error: Please give a mode and 2 .csv-files as argument\n")
    print("Mode: (0=printAverage, 1=printSystemUserTimes)\n")
    print("CSV: cothority.csv jvss.csv\n")
    exit(1)

option = sys.argv[1]
if option == "0":
    plotAvg(sys.argv[2], sys.argv[3])
elif option == "1":
    CoJVTime(sys.argv[2], sys.argv[3])
