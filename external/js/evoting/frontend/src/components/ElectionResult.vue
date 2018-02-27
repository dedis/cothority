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
            <v-layout row wrap>
              <v-flex xs5 md3>
                <p class="candidate">Foobar</p>
              </v-flex>
              <v-flex xs5 md7>
                <v-progress-linear color="success" value="90"></v-progress-linear>
              </v-flex>
              <v-flex xs2 md2 class="text-md-center">
                <span class="count">(180/200)</span>
              </v-flex>
            </v-layout>
            <v-layout>
              <v-flex xs5 md3>
                <p class="candidate">Foobar</p>
              </v-flex>
              <v-flex xs5 md7>
                <v-progress-linear value="10"></v-progress-linear>
              </v-flex>
              <v-flex xs2 md2 class="text-md-center">
                <span class="count">(20/200)</span>
              </v-flex>
            </v-layout>
          </v-container>
        </v-card-title>
      </v-card>
    </v-flex>
  </v-layout>
</template>

<script>
// import kyber from '@dedis/kyber-js'

// const curve = new kyber.curve.edwards25519.Curve()
export default {
  computed: {
    election () {
      return this.$store.state.loginReply.elections.find(e => {
        return btoa(e.id).replace('/\\/g', '-') === this.$route.params.id
      })
    }
  },
  data () {
    return {

    }
  },
  created () {
    const { socket } = this.$store.state
    socket.send('Reconstruct', 'ReconstructReply', {
      id: this.election.id,
      token: this.$store.state.loginReply.token
    })
      .then(data => {
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
