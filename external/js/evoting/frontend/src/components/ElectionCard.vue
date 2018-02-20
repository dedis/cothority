<template>
  <div class="flex sm12 md4 election-card">
    <v-card>
      <v-toolbar card dark>
        <v-toolbar-title class="white--text">{{ title }}</v-toolbar-title>
      </v-toolbar>
      <v-card-title class="election-card-title">
        <v-layout class="election-info-container" row>
          <v-flex class="election-info"><p><v-icon>alarm</v-icon> {{ endDate }}</p></v-flex>
          <v-flex class="election-info"><p><v-icon>account_box</v-icon> {{ creator }}</p></v-flex>
        </v-layout>
      </v-card-title>
      <v-card-actions>
        <v-btn :disabled="stage !== 0" :to="voteLink" color="primary">Vote</v-btn>
        <div v-if="$store.state.loginReply.admin">
          <v-btn v-on:click="shuffle" :disabled="stage >= 1" color="orange">Shuffle</v-btn>
          <v-btn v-on:click="decrypt" :disabled="stage === 0 || stage === 2" color="red">Decrypt</v-btn>
        </div>
        <v-spacer></v-spacer>
        <v-btn icon @click.native="show = !show">
          <v-icon>{{ show ? 'keyboard_arrow_down' : 'keyboard_arrow_up' }}</v-icon>
        </v-btn>
      </v-card-actions>
      <v-slide-y-transition>
        <v-card-text class="grey--text" v-show="show">
          {{ content }}
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
export default {
  props: {
    title: String,
    endDate: String,
    creator: String,
    content: String,
    stage: Number,
    id: String
  },
  methods: {
    shuffle (event) {
    },
    decrypt (event) {
    }
  },
  data () {
    return {
      show: false,
      voteLink: '/election/' + this.id + '/vote'
    }
  }
}
</script>
