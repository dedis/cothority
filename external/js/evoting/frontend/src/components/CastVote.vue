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
            <v-form v-model="valid" v-on:submit="submitHandler">
              <v-layout row wrap>
                <v-flex xs12>
                  <v-radio-group
                    label="Please select a candidate"
                    v-model="ballot"
                    :rules=[validateBallot]>
                    <v-radio
                      v-for="candidate in candidates(election.data)"
                      :key="candidate"
                      :label="`${candidate}`"
                      :value="`${candidate}`"
                    ></v-radio>
                  </v-radio-group>
                </v-flex>
                <v-flex xs12 class="text-xs-center">
                  <v-btn type="submit" :disabled="!valid || submitted" color="primary">Submit</v-btn>
                </v-flex>
              </v-layout>
            </v-form>
          </v-container>
        </v-card-title>
      </v-card>
    </v-flex>
    <v-snackbar
      :timeout="timeout"
      :color="snackbarColor"
      v-model="snackbar"
    >
      {{ snackbarText }}
      <v-btn dark flat @click.native="snackbar = false">Close</v-btn>
    </v-snackbar>
  </v-layout>
</template>

<script>
import kyber from '@dedis/kyber-js'
import { scipersToUint8Array } from '../utils'

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
    validateBallot (ballot) {
      return !!ballot || 'Please select a candidate'
    },
    submitHandler (e) {
      e.preventDefault()
      this.submitted = true

      const { key } = this.election
      // encrypt the ballot
      const embedMsg = scipersToUint8Array([this.ballot])
      console.log(embedMsg)
      const m = curve.point().embed(embedMsg)
      const r = curve.scalar().pick()
      // u = gr
      const u = curve.point().mul(r)
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
          console.log('Submitted Successfully')
          this.submitted = false
          this.snackbarColor = 'success'
          this.snackbarText = 'Your vote has been cast successfully'
          this.snackbar = true
        })
        .catch(e => {
          console.error(e)
          this.submitted = false
          this.snackbarColor = 'error'
          this.snackbarText = e.message
          this.snackbar = true
        })
      // show the encrypted message on success
    }
  },
  data () {
    return {
      ballot: null,
      valid: false,
      submitted: false,
      snackbar: false,
      snackbarColor: '',
      timeout: 6000,
      snackbarText: ''
    }
  }
}
</script>

<style>
.input-group label {
  overflow: visible;
}
</style>
