<template>
  <div class="flex sm12 md4 election-card">
    <v-card>
      <v-toolbar card dark>
        <v-toolbar-title class="white--text">{{ name }}</v-toolbar-title>
      </v-toolbar>
      <v-card-title class="election-card-name">
        <v-layout class="election-info-container" row>
          <v-flex class="election-info"><p><v-icon>alarm</v-icon> {{ end }}</p></v-flex>
          <v-flex class="election-info"><p><v-icon>account_box</v-icon> {{ creatorName }} ({{ creator }})</p></v-flex>
        </v-layout>
      </v-card-title>
      <v-card-actions>
        <v-layout row wrap>
        <v-flex v-if="stage === 0" xs5>
        <v-btn :disabled="disabled" :to="voteLink" color="primary">Vote</v-btn>
        </v-flex>
        <v-flex v-if="$store.state.loginReply.admin && stage === 0" class="text-xs-right" xs5>
          <v-btn :disabled="disabled" v-on:click.native="finalize" color="orange">Finalize</v-btn>
        </v-flex>
        <v-flex v-if="stage === 2" xs10>
          <v-btn :disabled="disabled" :to="resultLink" color="success">View Results</v-btn>
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
  </div>
</template>


<style>
.election-card {
  padding: 1rem;
}
</style>

<script>
import config from '@/config'
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
    finalize (event) {
      const { socket } = this.$store.state
      this.disabled = true
      const msg = {
        token: this.$store.state.loginReply.token,
        id: Uint8Array.from(atob(this.id.replace(/-/g, '/')).split(',').map(x => parseInt(x)))
      }
      socket.send('Shuffle', 'ShuffleReply', msg)
        .then(() => {
          return socket.send('Decrypt', 'DecryptReply', msg)
        })
        .then(() => {
          this.$store.commit('SET_SNACKBAR', {
            color: 'success',
            text: 'Election finalized',
            model: true,
            timeout: 6000
          })
          this.disabled = false
          const { sciper, signature } = this.$store.state.user
          const id = config.masterKey
          return socket.send('Login', 'LoginReply', {
            id,
            user: parseInt(sciper),
            signature: Uint8Array.from(signature)
          })
        })
        .then(response => {
          this.$store.commit('SET_LOGIN_REPLY', response)
          this.$router.push('/')
        })
        .catch(e => {
          this.$store.commit('SET_SNACKBAR', {
            color: 'error',
            text: e.message,
            model: true,
            timeout: 6000
          })
          this.disabled = false
        })
    }
  },
  data () {
    return {
      show: false,
      voteLink: `/election/${this.id}/vote`,
      resultLink: `/election/${this.id}/results`,
      disabled: false,
      creatorName: '',
      candidateNames: []
    }
  },
  created () {
    // creator
    if (this.creator in this.$store.state.names) {
      this.creatorName = this.$store.state.names[this.creator]
      return
    }
    this.$store.state.socket.send('LookupSciper', 'LookupSciperReply', {
      sciper: this.creator.toString()
    })
      .then(response => {
        this.creatorName = response.fullName
        // cache
        this.$store.state.names[this.creator] = this.creatorName
      })
  }
}
</script>
