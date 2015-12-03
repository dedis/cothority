__author__ = 'ligasser'

import matplotlib.pyplot as plt
import matplotlib.ticker
import csv
import sys
import math
import matplotlib.patches as mpatches
from matplotlib.legend_handler import HandlerLine2D, HandlerRegularPolyCollection
from mplot import MPlot
from stats import CSVStats

color1_light = 'lightgreen'
color1_dark = 'green'
color2_light = 'lightblue'
color2_dark = 'blue'
color3_light = 'yellow'
color3_dark = 'brown'
color4_light = 'pink'
color4_dark = 'red'

mplot = MPlot()
mplot.plotPrepareLogLog()

mplot.show_fig = True
jvss = CSVStats('test_naive_multi.csv').get_values('round_wall')
plt.plot(jvss.x, jvss.avg, label='JVSS', linestyle='-', marker='^', color=color2_dark, zorder=3)
mplot.plotFilledLegend(jvss.x, jvss.min, jvss.max, "min-max", color2_light, z=0)
mplot.plotEnd()