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
              <v-flex class="election-info"><p><v-icon>alarm</v-icon> {{ election.end }}</p></v-flex>
              <v-flex class="election-info"><p><v-icon>account_box</v-icon> {{ election.creator }}</p></v-flex>
            </v-layout>
            <v-layout>
              <v-flex xs12>
                <p><v-icon>comment</v-icon>{{ election.description }}</p>
              </v-flex>
            </v-layout>
            <v-layout
              v-for="(val, key) in counts"
              :key="key"
              row
              wrap>
              <v-flex xs5 md3>
                <p class="candidate">{{ key }}</p>
              </v-flex>
              <v-flex xs5 md7>
                <v-progress-linear color="success" :value="percentage(val, totalCount)"></v-progress-linear>
              </v-flex>
              <v-flex xs2 md2 class="text-md-center">
                <span class="count">({{ val }}, {{ totalCount }})</span>
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
    }
  },
  data () {
    const counts = {}
    const c = this.candidates(this.election.data)
    for (let i = 0; i < c.length; i++) {
      counts[c[i]] = 0
    }
    return {
      counts,
      totalCount: 0
    }
  },
  created () {
    const { socket } = this.$store.state
    console.log(this.election)
    socket.send('Reconstruct', 'ReconstructReply', {
      id: this.election.id,
      token: this.$store.state.loginReply.token
    })
      .then(data => {
        const { points } = data
        console.log(points)
        for (let i = 0; i < points.length; i++) {
          const point = curve.point()
          point.unmarshalBinary(points[i])
          const d = point.data()
          const sciper = d[0] + d[1] * (1 << 8) + d[2] * (1 << 16)
          this.counts[sciper] += 1
          this.totalCount += 1
        }
      })
      .catch(e => {
        console.error(e.message)
      })
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
