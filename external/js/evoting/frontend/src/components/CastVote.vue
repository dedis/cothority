<template>
  <v-layout row wrap>
    <v-flex sm12 offset-md3 md6>
      <v-card>
        <v-toolbar card dark :class="election.theme">
          <v-toolbar-title class="white--text">{{ election.name }}</v-toolbar-title>
          <v-spacer></v-spacer>
          <div v-if="election.moreInfo">
            <a class="election-info" target="_blank" :href="election.moreInfo"><v-icon>info</v-icon></a>
          </div>
        </v-toolbar>
        <v-card-title>
          <v-container fluid>
            <v-layout>
              <v-flex xs12>
                {{ election.subtitle }}
              </v-flex>
            </v-layout>
            <br>
            <v-form v-model="valid" v-on:submit="submitHandler">
              <v-layout row wrap>
                <v-flex xs12>
                  <p>In the following list, please select at most {{ election.maxChoices }} candidates. You may click on a name to see their motivation and presentation</p>
                    <v-checkbox
                      v-for="candidate in election.candidates"
                      :key="candidate"
                      :value="`${candidate}`"
                      v-model="ballot"
                      :rules=[validateBallot]
                      >
                      <template slot="label">
                        <a @click.stop target="_blank" :href="`https://people.epfl.ch/${candidate}`">{{ candidateNames[candidate] }}</a>
                      </template>
                    </v-checkbox>
                  </v-radio-group>
                </v-flex>
                <v-flex xs12 class="text-xs-center">
                  <v-btn type="submit" :disabled="!(valid && ballot.length !== 0) || submitted" color="primary">Submit</v-btn>
                </v-flex>
              </v-layout>
            </v-form>
          </v-container>
        </v-card-title>
      </v-card>
    </v-flex>
    <v-footer app>
      <v-layout row wrap>
        <v-flex xs6 text-xs-left>&copy; 2018 {{ election.footer.text }}</v-flex><v-flex text-xs-right>{{ election.footer.contactPhone }}, <a :href="`mailto:${election.footer.contactEmail}`">{{ election.footer.contactTitle }}</a></v-flex>
      </v-layout>
    </v-footer>
  </v-layout>
</template>

<script>
import kyber from '@dedis/kyber-js'
import { scipersToUint8Array, timestampToString } from '../utils'

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
    endDate (timestamp) {
      return timestampToString(timestamp, true)
    },
    validateBallot (ballot) {
      const { election } = this
      return ballot.length <= election.maxChoices || `Maximum ${election.maxChoices} allowed`
    },
    dateStr (timestamp) {
      return timestampToString(timestamp, true)
    },
    submitHandler (e) {
      e.preventDefault()
      this.submitted = true

      const { key } = this.election
      // encrypt the ballot
      const embedMsg = scipersToUint8Array(this.ballot)
      const m = curve.point().embed(embedMsg)
      const r = curve.scalar().pick()
      // u = gr
      const u = curve.point().mul(r, null)
      // v = m + yr
      const y = curve.point()
      y.unmarshalBinary(key)
      const yr = curve.point().mul(r, y)
      const v = curve.point().add(m, yr)

      // prepare and the message
      const castMsg = {
        token: this.$store.state.loginReply.token,
        id: this.election.id,
        ballot: {
          user: parseInt(this.$store.state.user.sciper),
          alpha: u.marshalBinary(),
          beta: v.marshalBinary()
        }
      }
      const { socket } = this.$store.state
      socket.send('Cast', 'CastReply', castMsg)
        .then(() => {
          this.submitted = false
          this.$store.commit('SET_SNACKBAR', {
            color: 'success',
            text: 'Your vote has been cast successfully',
            model: true,
            timeout: 6000
          })
          this.$router.push('/')
        })
        .catch(e => {
          this.submitted = false
          this.$store.commit('SET_SNACKBAR', {
            color: 'error',
            text: e.message,
            model: true,
            timeout: 6000
          })
        })
    }
  },
  data () {
    return {
      ballot: [],
      valid: false,
      submitted: false,
      creatorName: '',
      candidateNames: {}
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
    const scipers = this.election.candidates
    for (let i = 0; i < scipers.length; i++) {
      const sciper = scipers[i]
      this.candidateNames[sciper] = this.$store.state.names[sciper] || ''
      if (this.candidateNames[sciper]) {
        continue
      }
      this.$store.state.socket.send('LookupSciper', 'LookupSciperReply', {
        sciper: sciper.toString()
      })
        .then(response => {
          this.candidateNames = {...this.candidateNames, [sciper]: response.fullName}
          // cache
          this.$store.state.names[sciper] = this.candidateNames[sciper]
        })
    }
  },
  watch: {
    candidateNames: {
      deep: true,
      handler (val, oldVal) {}
    }
  }
}
</script>

<style>
.input-group label {
  overflow: visible;
}
</style>
