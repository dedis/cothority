import Vue from 'vue'
import Vuex from 'vuex'
import createPersistedState from 'vuex-persistedstate'
import rosterTOML from './public.toml'
import cothority from '@dedis/cothority'

Vue.use(Vuex)

const net = cothority.net
const roster = cothority.Roster.fromTOML(rosterTOML)

console.log('Creating new store')

const store = new Vuex.Store({
  state: {
    user: null,
    loginReply: null,
    socket: new net.RosterSocket(roster, 'evoting'),
    snackbar: {
      text: '',
      timeout: 6000,
      model: false,
      color: ''
    },
    names: {},
    now: Math.floor(new Date().getTime() / 1000)
  },
  getters: {
    isAuthenticated: state => {
      return state.user !== null
    },
    hasLoginReply: state => {
      return state.loginReply !== null
    },
    snackbar: state => {
      return state.snackbar
    }
  },
  mutations: {
    SET_LOGIN_REPLY (state, loginReply) {
      state.loginReply = loginReply
    },
    SET_USER (state, data) {
      state.user = data
    },
    SET_SNACKBAR (state, snackbar) {
      state.snackbar = snackbar
    },
    SET_NOW (state, now) {
      state.now = now
    }
  },
  plugins: [createPersistedState({ key: 'evoting', paths: ['user'] })]
})

export default store
