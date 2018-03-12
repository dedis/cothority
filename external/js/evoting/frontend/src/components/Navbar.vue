<template>
  <div>
  <v-toolbar
    app
  >
    <div class="logo">
      <a href="/"><img src="@/assets/epfl_logo.png"></a>
    </div>
    <v-toolbar-title v-text="title"></v-toolbar-title>
    <v-spacer></v-spacer>
    <v-toolbar-items>
      <v-menu offset-y>
        <v-btn slot="activator" icon><v-icon>translate</v-icon></v-btn>
        <v-list>
          <v-list-tile>
            <v-list-tile-title>English</v-list-tile-title>
          </v-list-tile>
          <v-list-tile>
            <v-list-tile-title>Français</v-list-tile-title>
          </v-list-tile>
        </v-list>
      </v-menu>
    </v-toolbar-items>
    <v-toolbar-items v-if="$store.getters.hasLoginReply" class="hidden-md-and-up">
      <v-menu>
        <v-btn slot="activator" icon><v-icon>more_vert</v-icon></v-btn>
        <v-list>
          <v-list-tile>
            <v-list-tile-avatar><v-icon>account_circle</v-icon></v-list-tile-avatar>
            <v-list-tile-content>
              <v-list-tile-title>{{ $store.state.user.name }}</v-list-tile-title>
            </v-list-tile-content>
          </v-list-tile>
          <v-list-tile to="/logout">
            <v-list-tile-avatar><v-icon>exit_to_app</v-icon></v-list-tile-avatar>
            <v-list-tile-content>
              <v-list-tile-title>Logout</v-list-tile-title>
            </v-list-tile-content>
          </v-list-tile>
        </v-list>
      </v-menu>
    </v-toolbar-items>
    <v-toolbar-items v-if="$store.getters.hasLoginReply" class="hidden-sm-and-down">
        <v-btn flat>{{ $store.state.user.name }}</v-btn>
        <v-btn to="/logout" flat><v-icon>exit_to_app</v-icon></v-btn>
    </v-toolbar-items>
  </v-toolbar>
  </div>
</template>

<style>
  .logo img {
    width: 108px;
    padding: 5px 0 0 0;
  }
</style>

<script>
export default {
  props: {
    title: String
  },
  data () {
    return {
    }
  }
}
</script>
