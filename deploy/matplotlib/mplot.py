# Plots the graph of one of the test-runs
# It takes the CSV-file as argument and shows the plot
# of the times used for each round

import matplotlib.pyplot as plt
import matplotlib.ticker
import csv
import sys
import math
import matplotlib.patches as mpatches
from matplotlib.legend_handler import HandlerLine2D, HandlerRegularPolyCollection


class MPlot:
    show_fig = True
    pngname = ""
    plt = None

    def __init__(self):
        vers = matplotlib.__version__
        if vers != "1.4.3":
            print "\nWrong matlib-version " + vers + ", please install 1.4.3"
            print "http://matplotlib.org/faq/installing_faq.html\n"
            exit(1)
        self.plt = plt

    # Adds a fill_between and the corresponding 'empty' plot to show up in
    # the legend
    def plotFilledLegend(self, stats, col1, col2, label, color, z=None):
        stats.reset_min_max()
        y1 = stats.update_values(col1)
        y2 = stats.update_values(col2)
        if z:
            fb = plt.fill_between(stats.x, y1, y2, facecolor=color, edgecolor='white', zorder=z)
        else:
            fb = plt.fill_between(stats.x, y1, y2, facecolor=color, edgecolor='white', zorder=3)
        # plt.plot([], [], '-', label=label, color=color, linewidth=10)

    # Takes one x and y1, y2 to stack y2 on top of y1. Does all the
    # calculation necessary to sum up everything
    def plotStacked(self, stats, col1, col2, label1, label2, color1, color2, ymin=None):
        stats.reset_min_max()
        y1 = stats.update_values(col1)
        y2 = stats.update_values(col2)
        if ymin == None:
            ymin = min(min(y1), min(y2))
        ymins = [ymin] * len(x)
        ysum = [sum(t) for t in zip(y1, y2)]
        self.plotFilledLegend(stats.x, y1, ysum, label2, color2)
        self.plotFilledLegend(stats.x, ymins, y1, label1, color1)


    # Takes one x and y1, y2 to stack y2 on top of y1. Does all the
    # calculation necessary to sum up everything
    def plotStackedBars(self, stats, col1, col2, label1, label2, color1, color2, ymin=None,
                    delta_x=0):
        stats.reset_min_max()
        y1 = stats.update_values(col1)
        y2 = stats.update_values(col2)
        width = [(t * 0.125 + delta_x * t * 0.018) for t in x]

        zero = [min(y1) for t in y1]
        xd = [t[0] + delta_x * t[1] for t in zip(stats.x, width)]
        y12 = [sum(t) for t in zip(y1, y2)]
        plt.bar(xd, y12, width, color=color1, bottom=y1, zorder=3, label=label1)
        plt.bar(xd, y1, width, color=color2, bottom=zero, zorder=3, label=label2)

    # Takes one x and y1, y2 to stack y2 on top of y1. Does all the
    # calculation necessary to sum up everything
    def plotStackedBarsHatched(self, stats, col1, col2, label, color, ymin=None,
                           delta_x=0):
        stats.reset_min_max()
        y1 = stats.update_values(col1)
        y2 = stats.update_values(col2)
        width = [(t * 0.18 + delta_x * t * 0.018) for t in x]

        zero = [min(y1) for t in y1]
        xd = [t[0] + ( delta_x - 0.5 ) * t[1] for t in zip(x, width)]
        y12 = [sum(t) for t in zip(y1, y2)]
        plt.bar(xd, y12, width, color=color, bottom=y1, zorder=3, hatch='//')
        return plt.bar(xd, y1, width, color=color, bottom=zero, zorder=3, label=label)


    # Puts the most used arguments for starting a plot with
    # LogLog by default.
    def plotPrepareLogLog(logx=True, logy=True):
        plt.clf()
        plt.ylabel('Total seconds over all rounds')
        plt.xlabel('Number of co-signing hosts')
        if logx:
            plt.xscale(u'log', basex=2)
        if logy:
            plt.yscale(u'log', basey=2)

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
            plt.show()
        else:
            print "Saving to", self.pngname
            plt.savefig(self.pngname)


    # Draws an arrow for out-of-bound data
    def arrow(self, text, x, top, color):
        plt.annotate(text, xy=(x, top), xytext=(x, top - 1),
                     arrowprops=dict(facecolor=color, frac=0.4, width=8, headwidth=20, edgecolor='white'),
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


