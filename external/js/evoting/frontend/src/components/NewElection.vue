<template>
  <v-layout row wrap>
    <v-flex sm12 offset-md3 md6>
      <v-card>
        <v-toolbar card dark>
          <v-toolbar-title class="white--text">New Election</v-toolbar-title>
        </v-toolbar>
        <v-container fluid>
          <v-form v-model="valid" v-on:submit="submitHandler">
          <v-layout row wrap>
            <v-flex xs12>
              <v-text-field
                label="Election Name"
                v-model="name"
                :counter=20
                prepend-icon="create"
                :rules=[validateName]
                required
              ></v-text-field>
            </v-flex>
            <v-flex xs12>
              <v-text-field
                label="Election Description"
                v-model="description"
                :counter=100
                prepend-icon="mode_comment"
                :rules=[validateDescription]
                required
              ></v-text-field>
            </v-flex>
            <v-flex xs12>
              <datetime-picker
                label="Start Time"
                :datetime="`${today} 00:00`"
                @input="updateStartTime"
                ></datetime-picker>
            </v-flex>
            <v-flex xs12>
              <datetime-picker
                label="End Time"
                :datetime="`${today} 23:59`"
                @input="updateEndTime"
                ></datetime-picker>
            </v-flex>
            <v-flex xs12>
              <v-text-field
                label="Max Choices"
                v-model="maxChoices"
                type="number"
                prepend-icon="format_list_numbered"
                :rules=[validateMaxChoices]
                required
              ></v-text-field>
            </v-flex>
            <v-flex xs12>
              <v-select
                label="Candidate Scipers"
                prepend-icon="filter_list"
                chips
                tags
                :rules=[validateSciper]
                clearable
                required
                v-model="candidateScipers"
              > 
                <template slot="selection" slot-scope="data">
                  <v-chip
                    label
                    close
                    @input="removeCandidateSciper(data.item)"
                    :selected="data.selected"
                  >
                    <strong>{{ data.item }}</strong>&nbsp;
                  </v-chip>
                </template>
              </v-select>
            </v-flex>
            <!--<v-flex xs12>
              <v-select
                label="Voter Groups"
                prepend-icon="filter_list"
                chips
                tags
                clearable
                :rules=[validateGroup]
                v-model.trim="groups"
              > 
                <template slot="selection" slot-scope="data">
                  <v-chip
                    close
                    label
                    @input="removeGroup(data.item)"
                    :selected="data.selected"
                  >
                    <strong>{{ data.item }}</strong>&nbsp;
                  </v-chip>
                </template>
              </v-select>
            </v-flex>-->
            <v-flex xs12>
              <v-select
                label="Voter Scipers"
                prepend-icon="filter_list"
                chips
                tags
                :rules=[validateVoterSciper]
                clearable
                required
                v-model="voterScipers"
              > 
                <template slot="selection" slot-scope="data">
                  <v-chip
                    label
                    close
                    @input="removeVoterSciper(data.item)"
                    :selected="data.selected"
                  >
                    <strong>{{ data.item }}</strong>&nbsp;
                  </v-chip>
                </template>
              </v-select>
            </v-flex>
            <v-flex xs12 class="text-xs-center">
              <v-btn type="submit" :disabled="!valid || submitted" color="primary">Create Election</v-btn>
            </v-flex>
          </v-layout>
        </v-form>
        </v-container>
      </v-card>
    </v-flex>
  </v-layout>
</template>

<script>
import config from '../config'
import DateTimePicker from './DateTimePicker'
import { timestampToString } from '@/utils'

export default {
  methods: {
    removeGroup (item) {
      this.groups.splice(this.groups.indexOf(item), 1)
      this.groups = [...this.groups]
    },
    removeCandidateSciper (item) {
      this.candidateScipers.splice(this.candidateScipers.indexOf(item), 1)
      this.candidateScipers = [...this.candidateScipers]
    },
    removeVoterSciper (item) {
      this.voterScipers.splice(this.voterScipers.indexOf(item), 1)
      this.voterScipers = [...this.voterScipers]
    },
    validateSciper (items) {
      if (items.length === 0) {
        return 'Atleast one sciper is required'
      }
      if (items.length < this.maxChoices) {
        return 'Please enter atleast as many candidates as specified in Max Choices'
      }
      const sciperFormat = /^\d{6}$/
      return sciperFormat.test(items[items.length - 1]) || 'Invalid Sciper'
    },
    validateVoterSciper (items) {
      if (items.length === 0) {
        return 'Atleast one sciper is required'
      }
      const sciperFormat = /^\d{6}$/
      return sciperFormat.test(items[items.length - 1]) || 'Invalid Sciper'
    },
    validateGroup (items) {
      if (items.length === 0) {
        return true
      }
      return /\w+/.test(items[items.length - 1]) || 'Invalid Group'
    },
    validateName (name) {
      return !!name || 'Name field is required'
    },
    validateDescription (description) {
      return !!description || 'Description field is required'
    },
    validateMaxChoices (maxChoices) {
      if (!maxChoices) {
        return 'Please enter the maximum votes allowed per ballot'
      }
      if (maxChoices <= 0) {
        return 'Max Choices should atleast be 1'
      }
      return maxChoices <= 9 || 'Max Choices can be atmost 9'
    },
    updateStartTime (dt) {
      this.start = dt
    },
    updateEndTime (dt) {
      this.end = dt
    },
    submitHandler (e) {
      e.preventDefault()
      this.submitted = true

      const openProto = {
        token: this.$store.state.loginReply.token,
        id: config.masterKey,
        election: {
          name: this.name,
          creator: parseInt(this.$store.state.user.sciper),
          users: this.voterScipers.map(e => parseInt(e)),
          description: this.description,
          start: Math.floor(this.start / 1000),
          end: Math.floor(this.end / 1000),
          candidates: this.candidateScipers.map(x => parseInt(x)),
          maxChoices: parseInt(this.maxChoices)
        }
      }
      const { socket } = this.$store.state
      socket.send('Open', 'OpenReply', openProto)
        .then(data => {
          this.submitted = false
          this.$router.push('/')
          this.$store.commit('SET_SNACKBAR', {
            color: 'success',
            text: 'New election created',
            timeout: 6000,
            model: true
          })
          // refresh the election list - TODO: replace with get elections message
          const { sciper, signature } = this.$store.state.user
          const id = config.masterKey
          return socket.send('Login', 'LoginReply', { id,
            user: parseInt(sciper),
            signature: Uint8Array.from(signature)
          })
        })
        .then(response => {
          this.$store.commit('SET_LOGIN_REPLY', response)
          this.$router.push('/')
        })
        .catch(e => {
          console.error(e)
          this.submitted = false
          this.$store.commit('SET_SNACKBAR', {
            color: 'error',
            text: e.message,
            timeout: 6000,
            model: true
          })
        })
    }
  },
  data () {
    const today = timestampToString(this.$store.state.now, false)
    return {
      name: null,
      end: new Date(`${today} 23:59:00`).getTime(),
      start: new Date(`${today} 00:00:00`).getTime(),
      description: null,
      modal: false,
      groups: [],
      voterScipers: [],
      candidateScipers: [],
      valid: false,
      submitted: false,
      today,
      maxChoices: null
    }
  },
  components: {
    'datetime-picker': DateTimePicker
  }
}
</script>
