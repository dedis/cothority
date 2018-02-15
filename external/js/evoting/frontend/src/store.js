import Vue from 'vue'
import Vuex from 'vuex'
import createPersistedState from 'vuex-persistedstate'

Vue.use(Vuex)

const store = new Vuex.Store({
  state: {
    user: null,
    loginReply: null
  },
  getters: {
    isAuthenticated: state => {
      return state.user !== null
    },
    hasLoginReply: state => {
      return state.loginReply !== null
    }
  },
  mutations: {
    SET_LOGIN_REPLY (state, data) {
      state.loginReply = data
    }
  },
  plugins: [createPersistedState({ key: 'evoting' })]
})

export default store
