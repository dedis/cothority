<template>
  <v-toolbar
    app
  >
    <v-toolbar-title v-text="title"></v-toolbar-title>
    <v-spacer></v-spacer>
    <div v-if='$store.getters.hasLoginReply'>
      <v-btn flat>{{ $store.state.user.name }}</v-btn>
      <v-btn to="/logout" icon><v-icon>exit_to_app</v-icon></v-btn>
    </div>
  </v-toolbar>
</template>


<script>
export default {
  props: {
    title: String
  }
}
</script>
