#!/bin/bash
egrep "(base-files)" snapshots_full.csv > snapshots_limited_1.csv 
egrep "(cron|file|less|sed|cron|tar)" snapshots_full.csv > snapshots_limited_6.csv
