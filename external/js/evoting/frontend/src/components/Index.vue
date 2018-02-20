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
          color="primary"
        >
          <v-icon>add</v-icon>
        </v-btn>
      </div>
      <div class="election-group">
        <h3>Active Elections</h3>
        <v-layout v-for="layout in active(elections)" class="election-cards" row wrap>
          <election-card
            v-for="election in layout" :key="election.title"
            :title="election.title"
            :endDate="election.endDate"
            :creator="election.creator"
            :content="election.content"
            :stage="election.stage"></election-card>
        </v-layout>
      </div>
      <div class="election-group">
        <h3>Shuffled Elections</h3>
        <v-layout v-for="layout in shuffled(elections)" class="election-cards" row wrap>
          <election-card
            v-for="election in layout" :key="election.title"
            :title="election.title"
            :endDate="election.endDate"
            :creator="election.creator"
            :content="election.content"
            :stage="election.stage"></election-card>
        </v-layout>
      </div>
      <div class="election-group">
        <h3>Aggregated Elections</h3>
        <v-layout v-for="layout in aggregated(elections)" class="election-cards" row wrap>
          <election-card
            v-for="election in layout" :key="election.title"
            :title="election.title"
            :endDate="election.endDate"
            :creator="election.creator"
            :content="election.content"
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
    tmp.push(e)
    if (i > 0 && i % 3 === 0) {
      res.push(tmp)
      tmp = []
    }
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
    shuffled: (elections) => {
      return createArray(elections.filter(e => {
        return e.stage === 1
      }))
    },
    aggregated: (elections) => {
      return createArray(elections.filter(e => {
        return e.stage === 2
      }))
    }
  },
  data () {
    return {
      elections: [
        {
          title: 'Election 1',
          creator: 'John Doe',
          endDate: '1st May, 2018',
          content: 'Foo bar baz',
          stage: 0
        },
        {
          title: 'Election 2',
          creator: 'Jane Doe',
          endDate: '1st May, 2018',
          content: 'Foo bar baz',
          stage: 0
        },
        {
          title: 'Election 3',
          creator: 'John Doe',
          endDate: '1st May, 2018',
          content: 'Foo bar baz',
          stage: 2
        },
        {
          title: 'Election 4',
          creator: 'John Doe',
          endDate: '1st May, 2018',
          content: 'Foo bar baz',
          stage: 2
        },
        {
          title: 'Election 5',
          creator: 'Jenna Doe',
          endDate: '1st May, 2018',
          content: 'Foo bar baz',
          stage: 0
        },
        {
          title: 'Election 6',
          creator: 'Jenna Doe',
          endDate: '1st May, 2018',
          content: 'Foo bar baz',
          stage: 0
        }
      ]
    }
  }
}
</script>
