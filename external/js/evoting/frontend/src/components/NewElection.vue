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
              <v-dialog
                ref="dialog"
                persistent
                v-model="modal"
                lazy
                full-width
                width="290px"
                :return-value.sync="end"
              >
                <v-text-field
                  slot="activator"
                  label="End date"
                  v-model="end"
                  prepend-icon="event"
                  :rules=[validateDate]
                  required
                  readonly
                ></v-text-field>
                <v-date-picker
                  v-model="end"
                  :min="today"
                  scrollable
                >
                  <v-spacer></v-spacer>
                  <v-btn flat color="primary" @click="modal = false">Cancel</v-btn>
                  <v-btn flat color="primary" @click="$refs.dialog.save(end)">OK</v-btn>
                </v-date-picker>
              </v-dialog>
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
                :rules=[validateSciper]
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
/*
const getLdapData = (groups, scipers) => {
  const client = new LdapClient({ url: `ldap://${config.ldap.hostname}` })

  const groupFilter = groups.reduce((accumulator, group) => {
    return accumulator + `(ou:dn:=${group})`
  }, '')

  const sciperFilter = scipers.reduce((accumulator, sciper) => {
    return accumulator + `(uniqueIdentifier=${sciper})`
  }, '')

  const opts = {
    filter: `(&(objectClass=person)(|${groupFilter}${sciperFilter}))`,
    scope: 'sub',
    attributes: ['uniqueIdentifier']
  }

  const base = 'o=epfl, c=ch'

  return client.search(base, opts)
}
*/
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
      const sciperFormat = /^\d{6}$/
      return sciperFormat.test(items[items.length - 1]) || 'Invalid Sciper'
    },
    validateGroup (items) {
      if (items.length === 0) {
        return true
      }
      return /\w+/.test(items[items.length - 1]) || 'Invalid Group'
    },
    validateDate (date) {
      return (!!date && new Date(date) >= new Date(this.today)) || 'Date is required'
    },
    validateName (name) {
      return !!name || 'Name field is required'
    },
    validateDescription (description) {
      return !!description || 'Description field is required'
    },
    submitHandler (e) {
      e.preventDefault()
      this.submitted = true

      // construct the protobuf
      // send request to conode
      // PROFIT
    }
  },
  data () {
    let today = new Date()
    today = today.getFullYear() + '-' + (today.getMonth() + 1).toString().padStart(2, '0') + '-' + today.getDate()
    console.log(today)
    return {
      name: null,
      end: null,
      description: null,
      modal: false,
      groups: [],
      voterScipers: [],
      candidateScipers: [],
      valid: false,
      submitted: false,
      today
    }
  }
}
</script>
