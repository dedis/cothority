<template>
  <v-layout row wrap>
    <v-flex sm12 offset-md3 md6>
      <v-card>
        <v-toolbar card dark :class="theme">
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
            <v-flex md6 xs12>
              <v-text-field
                label="Election Subtitle"
                v-model="subtitle"
                :counter=100
                prepend-icon="mode_comment"
                :rules=[validateSubtitle]
                required
              ></v-text-field>
            </v-flex>
            <v-flex md6 xs12>
              <v-text-field
                label="More Info Link"
                v-model="moreInfo"
                prepend-icon="info"
              ></v-text-field>
            </v-flex>
            <v-flex md6 xs12>
              <datetime-picker
                label="Start Time"
                :datetime="`${today} 00:00`"
                @input="updateStartTime"
                ></datetime-picker>
            </v-flex>
            <v-flex md6 xs12>
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
            <v-flex xs12>
              <upload-button prepend-icon="filter_list" title="Voters" :selectedCallback="parseVoterList">
              </upload-button>
            </v-flex>
            <v-flex md6 xs12>
              <v-select
                label="Department"
                :items="departments"
                v-model="theme"
                prepend-icon="color_lens"
                item-text="name"
                item-value="class"
                :rules=[validateTheme]
                required
              >
                <template slot="item" slot-scope="data">
                  <v-avatar
                    :size="24"
                    :class="data.item.class"
                  >
                  </v-avatar>
                  <v-list-tile-content>
                    <v-list-tile-title>{{ data.item.name }}</v-list-tile-title>
                  </v-list-tile-content>
                </template>
              </v-select>
            </v-flex>
            <v-flex md6 xs12>
              <v-text-field
                label="Footer Text"
                v-model="footerText"
                :counter=80
                prepend-icon="create"
              ></v-text-field>
            </v-flex>
            <v-flex md4 xs12>
              <v-text-field
                label="Contact Title"
                v-model="footerContactTitle"
                :counter=20
                prepend-icon="create"
              ></v-text-field>
            </v-flex>
            <v-flex md4 xs12>
              <v-text-field
                label="Contact Phone"
                v-model="footerContactPhone"
                prepend-icon="phone"
              ></v-text-field>
            </v-flex>
            <v-flex md4 xs12>
              <v-text-field
                label="Contact Email"
                v-model="footerContactEmail"
                :counter=50
                prepend-icon="email"
                :rules=[validateEmail]
              ></v-text-field>
            </v-flex>
            <v-flex xs12 class="text-xs-center">
              <v-btn type="submit" :disabled="!valid || submitted || voterScipers.length === 0" color="primary">Create Election</v-btn>
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
import UploadButton from './UploadButton'
import { timestampToString } from '@/utils'

export default {
  methods: {
    parseVoterList (file) {
      if (file == null || file.type !== 'text/plain') {
        // show snackbar
        return
      }
      const fr = new FileReader()
      fr.onload = event => {
        const { result } = event.target
        const scipersStrArr = result.trim().split('\n')
        if (scipersStrArr.length === 0) {
          // show snackbar
          console.error(new Error('Atleast one sciper is required'))
          return
        }
        for (let i = 0; i < scipersStrArr.length; i++) {
          if (!(/^\d{6}$/.test(scipersStrArr[i]))) {
            // show snackbar
            console.error(new Error(`Invalid sciper ${scipersStrArr[i]} at line ${i}`))
            return
          }
        }
        const scipers = scipersStrArr.map(x => parseInt(x))
        this.voterScipers = scipers
        this.$store.state.scipersReadFromFile = scipers.length
      }
      fr.readAsText(file)
    },
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
    validateTheme (theme) {
      const classes = this.departments.map(x => x.class)
      return classes.indexOf(theme) !== -1 || 'Invalid theme'
    },
    validateEmail (email) {
      return email === '' || /^\w+([.-]?\w+)*@\w+([.-]?\w+)*(\.\w{2,3})+$/.test(email) || 'Invalid email'
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
    validateSubtitle (subtitle) {
      return !!subtitle || 'Subtitle field is required'
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
          users: this.voterScipers,
          subtitle: this.subtitle,
          moreInfo: this.moreInfo,
          start: Math.floor(this.start / 1000),
          end: Math.floor(this.end / 1000),
          candidates: this.candidateScipers.map(x => parseInt(x)),
          maxChoices: parseInt(this.maxChoices),
          theme: this.theme,
          footer: {
            text: this.footerText,
            contactTitle: this.footerContactTitle,
            contactEmail: this.footerContactEmail,
            contactPhone: this.footerContactPhone
          }
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
      subtitle: null,
      modal: false,
      groups: [],
      voterScipers: [],
      candidateScipers: [],
      valid: false,
      submitted: false,
      today,
      moreInfo: '',
      maxChoices: null,
      departments: [
        { name: 'EPFL', class: 'epfl' },
        { name: 'ENAC', class: 'enac' },
        { name: 'SB', class: 'sb' },
        { name: 'STI', class: 'sti' },
        { name: 'IC', class: 'ic' },
        { name: 'SV', class: 'sv' },
        { name: 'CDM', class: 'cdm' },
        { name: 'CDH', class: 'cdh' },
        { name: 'INTER', class: 'inter' },
        { name: 'Associations', class: 'assoc' }
      ],
      theme: '',
      footerText: '',
      footerContactTitle: '',
      footerContactPhone: '',
      footerContactEmail: ''
    }
  },
  components: {
    'datetime-picker': DateTimePicker,
    'upload-button': UploadButton
  }
}
</script>

