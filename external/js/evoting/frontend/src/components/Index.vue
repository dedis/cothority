<template>
  <div v-if='$store.getters.hasLoginReply'>
    <div>
      <div v-if="$store.state.loginReply.admin">
        <v-btn
          fixed
          dark
          fab
          bottom
          right
          to="/election/new"
          color="primary"
        >
          <v-icon>add</v-icon>
        </v-btn>
      </div>
      <div class="election-group">
        <h3>Active Elections</h3>
        <v-layout
          v-for="(layout, idx) in active(elections)"
          :key="idx"
          class="election-cards"
          row
          wrap>
          <election-card
            v-for="election in layout" :key="election.id.toString()"
            :id="getId(election.id)"
            :name="election.name"
            :end="election.end"
            :start="election.start"
            :theme="election.theme"
            :creator="election.creator"
            :subtitle="election.subtitle"
            :moreInfo="election.moreInfo"
            :stage="election.stage"></election-card>
        </v-layout>
      </div>
      <div class="election-group">
        <h3>Finalized Elections</h3>
        <v-layout
          v-for="(layout, idx) in finalized(elections)"
          :key="idx"
          class="election-cards"
          row
          wrap>
          <election-card
            v-for="election in layout" :key="election.name"
            :id="getId(election.id)"
            :name="election.name"
            :end="election.end"
            :start="election.start"
            :theme="election.theme"
            :creator="election.creator"
            :subtitle="election.subtitle"
            :moreInfo="election.moreInfo"
            :stage="election.stage"></election-card>
        </v-layout>
      </div>
    </div>
  </div>
  <div v-else>
    <v-layout row wrap align-center>
      <v-flex xs12 class='text-xs-center'>
        <div v-if='$store.getters.hasLoginReply'>
          <p>Welcome, {{ $store.state.user.name }}</p>
        </div>
        <div v-else>
          <v-progress-circular :indeterminate='true' :size="50" />
        </div>
      </v-flex>
    </v-layout>
  </div>
</template>

<style>
.election-group {
  border-bottom: 1px solid #eee;
  padding: 1rem 0;
  margin: 0.5rem 0;
}

.election-cards {
  margin-left: -1rem;
}
</style>

<script>
import ElectionCard from './ElectionCard'

const createArray = filteredArray => {
  const res = []
  let tmp = []
  filteredArray.forEach((e, i) => {
    if (i > 0 && i % 3 === 0) {
      res.push(tmp)
      tmp = []
    }
    tmp.push(e)
  })
  if (tmp.length > 0) {
    res.push(tmp)
  }
  return res
}

export default {
  components: {
    'election-card': ElectionCard
  },
  methods: {
    active: (elections) => {
      return createArray(elections.filter(e => {
        return e.stage === 0
      }))
    },
    finalized: (elections) => {
      return createArray(elections.filter(e => {
        return e.stage === 2
      }))
    },
    getId: (id) => {
      return btoa(id).replace(/\\/g, '-')
    }
  },
  computed: {
    elections () {
      return this.$store.state.loginReply.elections
    }
  }
}
</script>
