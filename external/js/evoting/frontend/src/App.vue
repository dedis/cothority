<template>
  <v-app>
    <navbar :title="title" />
    <!--<main>-->
      <v-container class="root-container" fluid full-height>
        <router-view/>
        <v-snackbar
          :timeout="$store.getters.snackbar.timeout"
          :color="$store.getters.snackbar.color"
          v-model="$store.getters.snackbar.model"
        >
          {{ $store.getters.snackbar.text }}
          <v-btn dark flat @click.native="snackbar = false">Close</v-btn>
        </v-snackbar>
      </v-container>
    <!--</main>-->
    <v-footer :fixed="fixed" app>
      <span>&copy; 2018</span>
    </v-footer>
  </v-app>
</template>

<script>
import Navbar from './components/Navbar'
import config from '@/config'
export default {
  components: {
    'navbar': Navbar
  },
  data () {
    return {
      fixed: false,
      title: 'Evoting'
    }
  },
  mounted () {
    setInterval(() => {
      this.$store.commit('SET_NOW', Math.floor(new Date().getTime() / 1000))
    }, 60000)

    setInterval(() => {
      const { socket, user } = this.$store.state
      socket.send('Login', 'LoginReply', {
        id: config.masterKey,
        user: parseInt(user.sciper),
        signature: Uint8Array.from(user.signature)
      })
        .then((loginReply) => {
          this.$store.commit('SET_LOGIN_REPLY', loginReply)
        })
    }, 570000)
  },
  name: 'App'
}
</script>

<style scope>
.root-container {
  margin-top: 64px !important;
}
</style>
