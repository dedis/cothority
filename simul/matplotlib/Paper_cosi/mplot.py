# Plots the graph of one of the test-runs
# It takes the CSV-file as argument and shows the plot
# of the times used for each round

import matplotlib
matplotlib.use('TkAgg')
import matplotlib.pyplot as plt
import matplotlib.ticker
import csv

# Our CSVs have a space after the comma, so we need a new 'dialect', here
# called 'deploy'
csv.register_dialect('deploy', delimiter=',', doublequote=False, quotechar='', lineterminator='\n', escapechar='',
                     quoting=csv.QUOTE_NONE, skipinitialspace=True)

class MPlot:
    show_fig = True
    pngname = ""
    plt = None

    def __init__(self):
        vers = matplotlib.__version__
        if vers != "1.5.1":
            print "\nWrong matlib-version " + vers +", please install 1.5.1"
            print "http://matplotlib.org/faq/installing_faq.html\n"
            print "Or try the following\nsudo easy_install \"matplotlib == 1.5.1\"\n"
            exit(1)
        self.plt = plt
        self.resetMinMax()

    def readCSV(self, name):
        print 'Reading ' + name

    def resetMinMax(self):
        self.ymin = -1
        self.ymax = 0
        self.xmin = -1
        self.xmax = 0


    # Updates the xmin and xmax with the given values on the x-axis
    def updateX(self, *values):
        for v in values:
            if self.xmin == -1:
                self.xmin = min(v)
            else:
                self.xmin = min(self.xmin, min(v))
            self.xmax = max(self.xmax, max(v))

    # Updates the xmin and xmax with the given values on the y-axis
    def updateY(self, *values):
        for v in values:
            if self.ymin == -1:
                self.ymin = min(v)
            else:
                self.ymin = min(self.ymin, min(v))
            self.ymax = max(self.ymax, max(v))

    # Plots the Minimum, Maximum, Average on the same plot.
    def plotMMA(self, stats, values, plot_color, plot_z, args):
        val = stats.get_values(values)
        plt.plot(val.x, val.avg, **args)
        self.plotFilledLegend(val, "min-max", plot_color, z=plot_z)
        return val

    # Adds a fill_between and the corresponding 'empty' plot to show up in
    # the legend
    def plotFilledLegend(self, stats, label, color, z=None):
        x, y1, y2 = stats.x, stats.min, stats.max
        if z:
            fb = plt.fill_between(x, y1, y2, facecolor=color, edgecolor='white', zorder=z)
        else:
            fb = plt.fill_between(x, y1, y2, facecolor=color, edgecolor='white', zorder=3)

        self.updateX(x)
        self.updateY(y1, y2)
        # plt.plot([], [], '-', label=label, color=color, linewidth=10)

    # Takes one x and y1, y2 to stack y2 on top of y1. Does all the
    # calculation necessary to sum up everything
    def plotStacked(self, stats, col1, col2, label1, label2, color1, color2, ymin=None, values=0):
        stats.reset_min_max()
        y1 = stats.update_values(col1)
        y2 = stats.update_values(col2)
	if values > 0:
	    y1 = y1[0:values-1]
	    y2 = y2[0:values-1]
        if ymin == None:
            ymin = min(min(y1), min(y2))
        ymins = [ymin] * len(x)
        ysum = [sum(t) for t in zip(y1, y2)]
        self.plotFilledLegend(stats.x, y1, ysum, label2, color2)
        self.plotFilledLegend(stats.x, ymins, y1, label1, color1)


    # Takes one x and y1, y2 to stack y2 on top of y1. Does all the
    # calculation necessary to sum up everything
    def plotStackedBars(self, stats, values1, values2, label1, label2, color1, color2, ymin=None,
                    delta_x=0, values = 0):
        val1 = stats.get_values(values1)
        val2 = stats.get_values(values2)
        x = val1.x
        y1 = val1.avg
        y2 = val2.avg
       	if values > 0:
	    y1 = y1[0:values-1]
	    y2 = y2[0:values-1]
	    x = x[0:values-1]
	width = [(t * 0.125 + delta_x * t * 0.018) for t in x]

        zero = [min(y1) for t in y1]
        xd = [t[0] + delta_x * t[1] for t in zip(stats.x, width)]
        y12 = [sum(t) for t in zip(y1, y2)]
        plt.bar(xd, y12, width, color=color1, bottom=y1, zorder=3, label=label1)
        plt.bar(xd, y1, width, color=color2, bottom=zero, zorder=3, label=label2)

    # Takes one x and y1, y2 to stack y2 on top of y1. Does all the
    # calculation necessary to sum up everything
    def plotStackedBarsHatched(self, stats, values1, values2, label, color, ymin=None,
                           limit_values=None, delta_x=0):
        val1 = stats.get_values(values1)
        val2 = stats.get_values(values2)
        x = val1.x
        y1 = val1.avg
        y2 = val2.avg
        if limit_values != None:
            x = x[0:limit_values]
            y1 = y1[0:limit_values]
            y2 = y2[0:limit_values]
        width = [(t * 0.18 + delta_x * t * 0.018) for t in x]

        zero = [min(y1) for t in y1]
        xd = [t[0] + ( delta_x - 0.5 ) * t[1] for t in zip(x, width)]
        y12 = [sum(t) for t in zip(y1, y2)]
        plt.bar(xd, y12, width, color=color, bottom=y1, zorder=3, hatch='//')
        return plt.bar(xd, y1, width, color=color, bottom=ymin, zorder=3, label=label), val1, val2


    # Puts the most used arguments for starting a plot with
    # LogLog by default.
    def plotPrepareLogLog(self, logx=2, logy=2):
        plt.clf()
        plt.ylabel('Total seconds over all rounds')
        plt.xlabel('Number of co-signing hosts')
        if logx > 0:
            plt.xscale(u'log', basex=logx)
        if logy > 0:
            plt.yscale(u'log', basey=logy)

        ax = plt.axes()
        ax.yaxis.grid(color='gray', linestyle='dashed', zorder=0)
        ax.xaxis.set_major_formatter(matplotlib.ticker.ScalarFormatter(useOffset=False))
        ax.xaxis.set_zorder(5)
        sf = matplotlib.ticker.ScalarFormatter()
        sf.set_powerlimits((-10, 10))
        sf.set_scientific(False)
        # ax.yaxis.set_major_formatter(sf)
        # ax.xaxis.set_major_formatter(matplotlib.ticker.FormatStrFormatter('%2.2e'))
        ax.yaxis.set_major_formatter(matplotlib.ticker.FormatStrFormatter('%2.2f'))


    # Ends the plot and takes an extension for saving the png. If
    # show_fig is True, it will show the window instead.
    def plotEnd(self):
        if self.show_fig:
            print "Showing plot"
            plt.show()
        else:
            print "Saving to", self.pngname
            plt.savefig(self.pngname)

        self.resetMinMax()


    # Draws an arrow for out-of-bound data
    def arrow(self, text, x, top, dist, color):
        plt.annotate(text, xy=(x, top), xytext=(x, top - dist),
                     arrowprops=dict(facecolor=color, headlength=5, width=6, headwidth=10, edgecolor='white'),
                     horizontalalignment='center', )

    # If we want to remove a poly
    def delete_poly(self, poly):
        self.poly.remove()


    # For removing a line
    def delete_line(self, line):
        self.line[0].remove()
        if len(self.line) > 1:
            for i in range(1, 3):
                for l in self.line[i]:
                    l.remove()
