# NR>1 means skip the header line, when NR==1
# 11 = transactions
# 104 = round_wall_avg
NR==1 { split($0, csv, ","); print csv[12] "/" csv[105] }
NR > 1 { split($0, csv, ","); print csv[12]/csv[105] }
