<template>
  <v-layout row wrap align-center>
    <v-flex xs12 class='text-xs-center'>
      <div v-if='$store.getters.hasLoginReply'>
        <p>Welcome, {{ $store.state.user.name }}</p>
      </div>
      <div v-else>
        <v-progress-circular :indeterminate='true' :size="50" />
      </div>
    </v-flex>
  </v-layout>
</template>
