<!-- From https://github.com/jannhama/vuetify-upload-btn/blob/master/UploadButton.vue -->
<template>
  <div class="upload-input input-group">
    <div class="input-group__input">
      <i aria-hidden="true" class="icon material-icons input-group__prepend-icon">{{ prependIcon }}</i>
      <div class="btn btn-primary jbtn-file"> {{ title }}
        <input type="file" v-on:change="fileSelected">
      </div>
      <div class="upload-info flex">{{ fileName || 'No file selected' }}</div>
      <div class="sciper-info" v-if="$store.state.scipersReadFromFile !== 0">Read {{ $store.state.scipersReadFromFile }} scipers</div>
    </div>
  </div>
</template>

<script>
  export default {
    name: 'upload-button',
    props: {
      selectedCallback: Function,
      title: String,
      prependIcon: String
    },
    data () {
      return {
        fileName: ''
      }
    },
    created () {
      this.$store.state.scipersReadFromFile = 0
    },
    methods: {
      fileSelected (e) {
        if (this.selectedCallback) {
          if (e.target.files[0]) {
            this.fileName = e.target.files[0].name
            this.selectedCallback(e.target.files[0])
            console.log(this.$store.state.scipersReadFromFile)
          } else {
            this.selectedCallback(null)
          }
        }
      }
    }
  }
</script>

<style scoped>
  .jbtn-file {
    cursor: pointer;
    position: relative;
    overflow: hidden;
  }

  .jbtn-file input[type=file] {
    position: absolute;
    top: 0;
    right: 0;
    min-width: 100%;
    min-height: 100%;
    text-align: right;
    filter: alpha(opacity=0);
    opacity: 0;
    outline: none;
    cursor: inherit;
    display: block;
  }

  .upload-info {
    display: flex;
    justify-content: left;
    align-items: center;
  }

  .sciper-info {
    display: flex;
    justify-content: right;
    align-items: center;
    text-align: right;
  }

  .upload-input {
    margin-bottom: 10px;
  }
</style>

