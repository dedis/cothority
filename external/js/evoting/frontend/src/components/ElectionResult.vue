<template>
  <v-layout row wrap>
    <v-flex sm12 offset-md3 md6>
      <v-card>
        <v-toolbar card dark>
          <v-toolbar-title class="white--text">{{ election.name }}</v-toolbar-title>
        </v-toolbar>
        <v-card-title>
          <v-container fluid>
            <v-layout class="election-info-container" row>
              <v-flex class="election-info"><p><v-icon>alarm</v-icon> {{ endDate(election.end) }}</p></v-flex>
              <v-flex class="election-info"><p><v-icon>account_box</v-icon> {{ creatorName }} ({{ election.creator }})</p></v-flex>
            </v-layout>
            <v-layout>
              <v-flex xs12>
                <p><v-icon>comment</v-icon>{{ election.description }}</p>
              </v-flex>
            </v-layout>
            <v-layout
              v-for="(val, idx) in sortCounts(counts)"
              :key="val.sciper"
              row
              wrap>
              <v-flex xs5 md3>
                <p class="candidate">{{ candidateNames[val.sciper] }}</p>
              </v-flex>
              <v-flex xs5 md7>
                <v-progress-linear :color="colors[idx % colors.length]" :value="percentage(val.count, totalCount)"></v-progress-linear>
              </v-flex>
              <v-flex xs2 md2 class="text-md-center">
                <span class="count">({{ val.count }}/{{ totalCount }})</span>
              </v-flex>
            </v-layout>
          </v-container>
        </v-card-title>
      </v-card>
    </v-flex>
  </v-layout>
</template>

<script>
import kyber from '@dedis/kyber-js'
import { timestampToString } from '@/utils'

const curve = new kyber.curve.edwards25519.Curve()
export default {
  computed: {
    election () {
      return this.$store.state.loginReply.elections.find(e => {
        return btoa(e.id).replace('/\\/g', '-') === this.$route.params.id
      })
    }
  },
  methods: {
    candidates (data) {
      const arr = []
      for (let i = 0; i < data.length; i += 3) {
        const num = data[i] + data[i + 1] * (1 << 8) + data[i + 2] * (1 << 16)
        arr.push(num)
      }
      return arr
    },
    percentage (num, den) {
      return num / den * 100
    },
    endDate (timestamp) {
      return timestampToString(timestamp, true)
    },
    sortCounts (counts) {
      const arr = []
      for (let sciper in counts) {
        arr.push({ sciper, count: counts[sciper] })
      }
      return arr.sort((a, b) => b.count - a.count)
    }
  },
  data () {
    return {
      counts: {},
      totalCount: 0,
      creatorName: '',
      candidateNames: {},
      colors: [
        'green accent-4',
        'green accent-3',
        'green accent-2',
        'yellow lighten-2',
        'yellow lighten-1',
        'yellow',
        'yellow darken-1',
        'yellow darken-2',
        'yellow darken-3',
        'amber',
        'amber darken-1',
        'amber darken-2',
        'amber darken-3',
        'amber darken-4',
        'orange',
        'orange darken-1',
        'orange darken-2',
        'orange darken-3',
        'orange darken-4'
      ]
    }
  },
  created () {
    if (this.election.creator in this.$store.state.names) {
      this.creatorName = this.$store.state.names[this.election.creator]
    } else {
      this.$store.state.socket.send('LookupSciper', 'LookupSciperReply', {
        sciper: this.election.creator.toString()
      })
        .then(response => {
          this.creatorName = response.fullName
          // cache
          this.$store.state.names[this.creator] = this.creatorName
        })
    }
    const c = this.candidates(this.election.candidates)
    for (let i = 0; i < c.length; i++) {
      this.counts[c[i]] = 0
    }
    const scipers = this.candidates(this.election.candidates)
    for (let i = 0; i < scipers.length; i++) {
      const sciper = scipers[i]
      this.candidateNames[sciper] = this.$store.state.names[sciper] || null
      if (this.candidateNames[sciper]) {
        continue
      }
      console.log(`Looking up ${sciper}`)
      this.$store.state.socket.send('LookupSciper', 'LookupSciperReply', {
        sciper: sciper.toString()
      })
        .then(response => {
          this.candidateNames = {...this.candidateNames, [sciper]: response.fullName}
          // cache
          this.$store.state.names[sciper] = this.candidateNames[sciper]
        })
    }
    const { socket } = this.$store.state
    socket.send('Reconstruct', 'ReconstructReply', {
      id: this.election.id,
      token: this.$store.state.loginReply.token
    })
      .then(data => {
        const { points } = data
        for (let i = 0; i < points.length; i++) {
          const point = curve.point()
          point.unmarshalBinary(points[i])
          const d = point.data()
          for (let j = 0; j < d.length; j += 3) {
            const sciper = d[j] + d[j + 1] * (1 << 8) + d[j + 2] * (1 << 16)
            this.counts[sciper] += 1
          }
          this.totalCount += d.length / 3
        }
      })
      .catch(e => {
        console.error(e.message)
      })
  },
  watch: {
    candidateNames: {
      deep: true,
      handler (val, oldVal) {}
    }
  }
}
</script>

<style scoped>
.candidate {
  line-height: 30px;
}

.count {
  line-height: 30px;
  font-size: 12px;
  font-weight: 500;
  padding: 0 5px;
}
</style>
