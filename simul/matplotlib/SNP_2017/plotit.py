#!/usr/bin/env python
# Plots the graph of one of the test-runs
# It takes the CSV-file as argument and shows the plot
# of the times used for each round

import os

os.environ["LC_ALL"] = "en_US.UTF-8"
os.environ["LANG"] = "en_US.UTF-8"

import sys

sys.path.insert(1, '.')
from mplot import MPlot
from stats import CSVStats
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
import numpy as np

import csv


def plotData(data, name,
             xlabel="Number of witnesses", ylabel="Signing round latency in seconds",
             xticks=[], loglog=[2, 2], xname="hosts",
             legend_pos="lower right",
             yminu=0, ymaxu=0,
             xminu=0, xmaxu=0,
             title="", read_plots=True,
             csv_column="round_wall"):
    mplot.plotPrepareLogLog(loglog[0], loglog[1])
    if read_plots:
        plots = read_csvs_xname(xname, *data[0])
    else:
        plots = data[0]

    ranges = []
    data_label = []
    plot_show(name)

    for index, label in enumerate(data[1]):
        data_label.append([plots[index], label])
        ranges.append(
            mplot.plotMMA(plots[index], csv_column, colors[index][0], 4,
                          dict(label=label, linestyle='-', marker='o',
                               color=colors[index][1], zorder=5)))

    # Make horizontal lines and add arrows for JVSS
    xmin, xmax, ymin, ymax = CSVStats.get_min_max(*ranges)
    if yminu != 0:
        ymin = yminu
    if ymaxu != 0:
        ymax = ymaxu
    if xminu != 0:
        xmin = xminu
    if xmaxu != 0:
        xmax = xmaxu
    plt.ylim(ymin, ymax)
    plt.xlim(xmin, xmax)
    plt.xlabel(xlabel)
    plt.ylabel(ylabel)

    plt.legend(loc=legend_pos)
    plt.axes().xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    if len(xticks) > 0:
        ax = plt.axes()
        ax.set_xticks(xticks)
    if title != "":
        plt.title(title)
    mplot.plotEnd()
    return data_label


def arrow(x, y, label, dx=1., dy=1., text_align='left'):
    plt.annotate(label, xy=(x + dx / 10, y + dy / 10),
                 xytext=(x + dx / 2, y + dy / 2),
                 horizontalalignment=text_align,
                 arrowprops=dict(facecolor='black', headlength=5, width=0.1,
                                 headwidth=8))


def plotVerify():
    plotData(
        [['swup_verification'],
         ['PGP key verification']],
        'swup_verification',
        loglog=[10, 10],
        xname="pgpkeys",
        csv_column="verification_wall",
        xlabel="Number of PGP-signatures on release",
        ylabel="Time to verify PGP-signatures (sec)"
    )

class CustomObject(object):
    pass

class CustomObjectHandler(object):
    def legend_artist(self, legend, orig_handle, fontsize, handlebox):

        x0, y0 = handlebox.xdescent, handlebox.ydescent
        width, height = handlebox.width, handlebox.height

        patch_CPU = mpatches.Rectangle([x0, y0 ], width/2+2, height,
                                   color='white', ec='black', hatch='//',
                                   transform=handlebox.get_transform())
        patch_WALL = mpatches.Rectangle([x0 + width/2 + 4, y0], width/2, height,
                                        color='white', ec='black',
                                        transform=handlebox.get_transform())
        handlebox.add_artist(patch_CPU)
        handlebox.add_artist(patch_WALL)

        return patch_CPU

def plotSBCreation():
    plots = read_csvs("swup_create_short")[0]
    plot_show("swup_create")
    mplot.plotPrepareLogLog(0, 10)
    width = 0.2
    x = np.arange(len(plots.x))
    zs = np.zeros(len(x))
    os = np.ones(len(x))
    vls = [['verification', 'Dev-signature verification'],
           ['swup_timestamp', 'Creating timestamp'],
           ['add_block', 'Collective signing'],
           ['build', 'Reproducible build']]

    handles = []
    labels = []
    for index, vl in enumerate(vls):
        value, label = vl
        usr = plots.get_values(value+"_user")
        sys = plots.get_values(value+"_system")
        ycpu = usr.avg + sys.avg
        wall = plots.get_values(value+"_wall")
        ywall = wall.avg
        yerr = [[ ycpu - sys.min - usr.min, sys.max + usr.max - ycpu ],
                [ ywall - wall.min, wall.max - ywall ]]
        if value == 'build':
            ycpu = os * ycpu[0]
            ywall = os * ywall[0]
            yerr[0][0] = [ycpu[0] - 60]
            yerr[1][0] = [ywall[0] - 60]
            for i in [0,1,2,3]:
                yerr[i/2][i%2] = os * yerr[i/2][i%2][0]

        plt.bar(x+(index-1.5)*width, ycpu, width/2, color=colors[index][0],
                hatch='//', yerr=yerr[0], error_kw=dict(ecolor='black'))
        h = plt.bar(x+(index-1)*width, ywall, width/2, color=colors[index][0],
                    yerr=yerr[1], error_kw=dict(ecolor='black'))
        handles.append(h)
        labels.append(label)

    labels.insert(0, "CPU / Wall")

    total = plots.get_values('overall_nobuild_wall').avg + ywall
    xtotal = x+width/2
    xtotal = np.concatenate(([xtotal[0]-1], xtotal, [xtotal[-1]+1]))
    total = np.concatenate(([total[0]], total, [total[-1]]))
    print total, xtotal
    oa_wall = plt.plot(xtotal, total, color=colors[len(vls)][1], marker='x')
    labels.insert(0, "Wall-total over all nodes")
    plt.xticks(x + width / 2, [3, 15, 127])


    plt.legend(oa_wall + [CustomObject()] + handles , labels, loc='upper center',
                prop={'size':10}, handler_map={CustomObject: CustomObjectHandler()},
                ncol=2)
    plt.ylim(0.001, 100000)
    plt.xlim(x[0]-2*width, x[-1]+3*width)
    plt.ylabel("Average time spent on each node per package (sec)")
    mplot.plotEnd()


def plotBuildCDF():
    files = [
        # ['repro_builds_essential', 'Debian essential packages'],
             ['repro_builds_required', 'Required (27)'],
             ['repro_builds_random', 'Random (50)'],
             ['repro_builds_popular', 'Popular (50)'],
             ]

    mplot.plotPrepareLogLog(0, 0)
    plot_show("repro_build_cdf")

    markers = "xso"
    for index, fl in enumerate(files):
        file, label = fl
        values = read_csv_pure(file, "", "cpu_user_time") + \
            read_csv_pure(file, "", "cpu_system_time")
        X = np.sort(values)
        while X[0] < 60.0:
            X = np.delete(X, 0)
        X /= 60
        Y = np.linspace(0, 100, len(X))

        plt.plot(X, Y, label=label, linestyle='-', marker=markers[index],
                 markevery=2, color=colors[index][1])

    plt.xlabel("Time (minutes)")
    plt.ylabel("% of Packages Built")

    plt.legend(loc='lower right', title='Debian package sets')
    ax = plt.axes()
    ax.xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    ax.grid(which='both')
    ax.grid(which='minor', alpha=0.2)
    ax.grid(which='major', alpha=0.5)
    plt.xticks(np.linspace(0,10,11))
    plt.yticks(np.linspace(0,100,11))
    plt.xlim(1,10)
    plt.annotate("Aptitude: 13'", xy=(10, 93),
                 xytext=(8, 93),
                 verticalalignment='center',
                 arrowprops=dict(facecolor=colors[2][1], ec=colors[2][1],
                                 headlength=5, width=0.1,
                                 headwidth=8))
    plt.annotate("Perl: 28'", xy=(10, 88),
                 xytext=(8, 88),
                 verticalalignment='center',
                 arrowprops=dict(facecolor=colors[0][1], ec=colors[0][1],
                                 headlength=5, width=0.1,
                                 headwidth=8))
    mplot.plotEnd()


def plotBW():
    mplot.plotPrepareLogLog(0, 10)
    plot_show('update_bandwidth')

    data = read_csvs_xname('frequency','swup_random_update_1_1')[0]
    bw_binaries = np.array(data.columns['download_binary_sum'])
    plt.plot(data.x, bw_binaries, label="Only binary download", marker='.')

    markers = "sxo+"
    for index, sparam in enumerate(['1_1', '4_5', '3_5', '2_5']):
        data = read_csvs_xname('frequency','swup_random_update_' + sparam)[0]
        bw_sc = np.array(data.columns['client_bw_swupdate_tx_sum']) + \
                    np.array(data.columns['client_bw_swupdate_rx_sum'])
        b, h = sparam.split("_")
        label = 'Skipchain $\mathcal{S}_{' + b + '}^{' + h + '}$ + metadata size'
        plt.plot(data.x, bw_sc, label=label, marker=markers[index])

    plt.xlabel("Client update frequency")
    plt.xticks(data.x)
    ax = plt.axes()
    ax.xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    plt.ylabel("Data and metadata bandwidth costs over 500 days")
    plt.xlim(1, 256)
    plt.ylim(2e6, 2e9)
    plt.legend(loc=(0.4,0.2))
    mplot.plotEnd()

def plotRandHerdSetup():
    plots = read_csvs(snp17rhound, snp17rherd, snp17cosi)
    plot_show("swup_create")
    mplot.plotPrepareLogLog(0, 10)
    width = 0.2
    x = np.arange(len(plots.x))
    rhound = np.zeros(len(plots.x), len(groupSizes))
    rherd = np.zeros(len(plots.x), len(groupSizes))
    cosi = np.zeros(len(plots.x), len(groupSizes))
    for hosts in x:
        for groupSize in groupSizes:
            rhound[hosts][groupSize] = \
                plots[0].get_values_filtered('tgen_randhound_wall', 'groupsize', groupSize)
            rherd[hosts][groupSize] = \
                plots[1].get_values_filtered('setup_wall', 'groupsize', groupSize)
            cosi[hosts][groupSize] = \
                plots[2].get_values('round_wall')
    os = np.ones(len(x))
    vls = [['verification', 'Dev-signature verification'],
           ['swup_timestamp', 'Creating timestamp'],
           ['add_block', 'Collective signing'],
           ['build', 'Reproducible build']]

    handles = []
    labels = []
    for index, vl in enumerate(vls):
        value, label = vl
        usr = plots.get_values(value+"_user")
        sys = plots.get_values(value+"_system")
        ycpu = usr.avg + sys.avg
        wall = plots.get_values(value+"_wall")
        ywall = wall.avg
        yerr = [[ ycpu - sys.min - usr.min, sys.max + usr.max - ycpu ],
                [ ywall - wall.min, wall.max - ywall ]]
        if value == 'build':
            ycpu = os * ycpu[0]
            ywall = os * ywall[0]
            yerr[0][0] = [ycpu[0] - 60]
            yerr[1][0] = [ywall[0] - 60]
            for i in [0,1,2,3]:
                yerr[i/2][i%2] = os * yerr[i/2][i%2][0]

        plt.bar(x+(index-1.5)*width, ycpu, width/2, color=colors[index][0],
                hatch='//', yerr=yerr[0], error_kw=dict(ecolor='black'))
        h = plt.bar(x+(index-1)*width, ywall, width/2, color=colors[index][0],
                    yerr=yerr[1], error_kw=dict(ecolor='black'))
        handles.append(h)
        labels.append(label)

    labels.insert(0, "CPU / Wall")

    total = plots.get_values('overall_nobuild_wall').avg + ywall
    xtotal = x+width/2
    xtotal = np.concatenate(([xtotal[0]-1], xtotal, [xtotal[-1]+1]))
    total = np.concatenate(([total[0]], total, [total[-1]]))
    print total, xtotal
    oa_wall = plt.plot(xtotal, total, color=colors[len(vls)][1], marker='x')
    labels.insert(0, "Wall-total over all nodes")
    plt.xticks(x + width / 2, [3, 15, 127])


    plt.legend(oa_wall + [CustomObject()] + handles , labels, loc='upper center',
               prop={'size':10}, handler_map={CustomObject: CustomObjectHandler()},
               ncol=2)
    plt.ylim(0.001, 100000)
    plt.xlim(x[0]-2*width, x[-1]+3*width)
    plt.ylabel("Average time spent on each node per package (sec)")
    mplot.plotEnd()

def plotRandHerdRound():

def plotRandHound():

def plotBandwidth():


# Colors for the Cothority
colors = [['lightgreen', 'green'],
          ['lightblue', 'blue'],
          ['yellow', 'brown'],
          ['pink', 'red'],
          ['pink', 'red']]
mplot = MPlot()


def plot_show(file):
    if write_file:
        mplot.pngname = file


def read_csvs_xname(xname, *values):
    stats = []
    for a in values:
        file = a + '.csv'
        stats.append(CSVStats(file, xname))
    return stats


def read_csvs(*values):
    return read_csvs_xname("hosts", *values)


def read_csv_pure(file, xname, yname):
    ret = {}
    with open(file+'.csv') as csvfile:
        reader = csv.DictReader(csvfile)
        x = 0
        for row in reader:
            if xname != "":
                x = row[xname]
            else:
                x += 1

            ret[x] = float(row[yname])

    return np.array(ret.values())

# Write to file
write_file = True
# file_extension = 'eps'
# Show figure
# mplot.show_fig = True
mplot.show_fig = False

snp17rhound = "snp17_randhound_small"
snp17rherd = "snp17_randherd_small"
snp17cosi = "snp17_cosi_small"
groupSizes = [16,64]

# Call all plot-functions
plotRandHerdSetup()
# plotRandHerdRound()
# plotRandHound()
# plotBandwidth()