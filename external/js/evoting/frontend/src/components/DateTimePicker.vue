<template>
  <v-menu
    lazy
    :close-on-content-click="false"
    v-model="menu"
    transition="v-scale-transition"
    offset-y
    >
    <v-text-field
      slot="activator"
      :label="label"
      v-model="actualDatetime"
      prepend-icon="event"
      readonly
      ></v-text-field>
    <v-tabs v-model="selectedTab" ref="tabs" grow>
      <v-tab href="#calendar">
        <v-icon>event</v-icon>
      </v-tab>
      <v-tab href="#timer">
        <v-icon>access_time</v-icon>
      </v-tab>

      <v-tab-item
        id="calendar">
        <v-date-picker
          v-model="dateModel"
          no-title
          actions
          :min="today"
          @input="checkHours"
          ></v-date-picker>
      </v-tab-item>
      <v-tab-item
        id="timer">
        <v-time-picker
          ref="timer"
          v-model="timeModel"
          actions
          @input="checkMinutes"
          ></v-time-picker>
      </v-tab-item>

    </v-tabs>
  </v-menu>
</template>

<script>
export default {
  props: {
    datetime: {
      type: String,
      required: true
    },
    label: {
      type: String,
      default: ''
    }
  },
  data () {
    let today = new Date()
    today = `${today.getFullYear()}-${(today.getMonth() + 1).toString().padStart(2, '0')}-${today.getDate().toString().padStart(2, '0')}`
    return {
      dateModel: '',
      timeModel: '',
      menu: false,
      selectedTab: 'calendar',
      today
    }
  },
  watch: {
    menu (val) {
      if (val) {
        this.selectedTab = 'calendar'
        if (this.$refs.timer) {
          this.$refs.timer.selectingHour = true
        }
      }
    }
  },
  computed: {
    actualDatetime () {
      return `${this.dateModel} ${this.timeModel}:00`
    }
  },
  methods: {
    checkMinutes (val) {
      if (this.$refs.timer.selectingHour === false) {
        this.timeModel = val
        this.$refs.timer.selectingHour = true
        this.selectedTab = 'calendar'
        this.menu = false
        this.$emit('input', new Date(this.actualDatetime).getTime())
      }
    },
    checkHours (val) {
      this.dateModel = val
      this.selectedTab = 'timer'
    }
  },
  created () {
    [this.dateModel, this.timeModel] = this.datetime.split(' ')
  }
}
</script>
