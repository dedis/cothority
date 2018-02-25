<template>
  <div class="flex sm12 md4 election-card">
    <v-card>
      <v-toolbar card dark>
        <v-toolbar-title class="white--text">{{ name }}</v-toolbar-title>
      </v-toolbar>
      <v-card-title class="election-card-name">
        <v-layout class="election-info-container" row>
          <v-flex class="election-info"><p><v-icon>alarm</v-icon> {{ end }}</p></v-flex>
          <v-flex class="election-info"><p><v-icon>account_box</v-icon> {{ creator }}</p></v-flex>
        </v-layout>
      </v-card-title>
      <v-card-actions>
        <v-layout row wrap>
        <v-flex v-if="stage === 0" xs5>
        <v-btn :disabled="disabled" :to="voteLink" color="primary">Vote</v-btn>
        </v-flex>
        <v-flex v-if="$store.state.loginReply.admin && stage === 0" class="text-xs-right" xs5>
          <v-btn :disabled="disabled" v-on:click.native="shuffle" color="orange">Shuffle</v-btn>
        </v-flex>
        <v-flex v-if="$store.state.loginReply.admin && stage === 1" xs5>
          <v-btn :disabled="disabled" v-on:click.native="decrypt" color="red">Decrypt</v-btn>
        </v-flex>
        <v-spacer></v-spacer>
        <v-flex xs2 class="text-xs-right">
        <v-btn icon @click.native="show = !show">
          <v-icon>{{ show ? 'keyboard_arrow_down' : 'keyboard_arrow_up' }}</v-icon>
        </v-btn>
        </v-flex>
        </v-layout>
      </v-card-actions>
      <v-slide-y-transition>
        <v-card-text class="grey--text" v-show="show">
          {{ description }}
        </v-card-text>
      </v-slide-y-transition>
    </v-card>
    <v-snackbar
      :timeout="timeout"
      :color="snackbarColor"
      v-model="snackbar"
    >
      {{ snackbarText }}
      <v-btn dark flat @click.native="snackbar = false">Close</v-btn>
    </v-snackbar>
  </div>
</template>


<style>
.election-card {
  padding: 1rem;
}
</style>

<script>
export default {
  props: {
    name: String,
    end: String,
    creator: Number,
    description: String,
    stage: Number,
    id: String
  },
  methods: {
    shuffle (event) {
      console.log('Shuffling')
      this.disabled = true
      const shuffleMsg = {
        token: this.$store.state.loginReply.token,
        id: Uint8Array.from(atob(this.id).split(',').map(x => parseInt(x)))
      }
      this.$store.state.socket.send('Shuffle', 'ShuffleReply', shuffleMsg)
        .then(() => {
          this.snackbarColor = 'success'
          this.snackbarText = 'Ballots have been shuffled'
          this.snackbar = true
          this.disabled = false
          this.stage = 1
        })
        .catch(e => {
          this.snackbarColor = 'error'
          this.snackbarText = e.message
          this.snackbar = true
          this.disabled = false
        })
    },
    decrypt (event) {
      console.log('Decrypting')
      this.disabled = true
      const decryptMessage = {
        token: this.$store.state.loginReply.token,
        id: Uint8Array.from(atob(this.id).split(',').map(x => parseInt(x)))
      }
      this.$store.state.socket.send('Decrypt', 'DecryptReply', decryptMessage)
        .then(() => {
          this.snackbarColor = 'success'
          this.snackbarText = 'Ballots have been decrypted'
          this.snackbar = true
          this.disabled = false
        })
        .catch(e => {
          this.snackbarColor = 'error'
          this.snackbarText = e.message
          this.snackbar = true
          this.disabled = false
        })
    }
  },
  data () {
    return {
      show: false,
      voteLink: '/election/' + this.id + '/vote',
      snackbar: false,
      snackbarColor: '',
      snackbarText: '',
      timeout: 6000,
      disabled: false
    }
  }
}
</script>
