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
  name: 'App'
}
</script>

<style scope>
.root-container {
  margin-top: 60px;
}
</style>
