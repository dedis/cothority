const path = require("path");
const UglifyJsPlugin = require("uglifyjs-webpack-plugin");

const nodeConfig = {
  target: "node",
  entry: "./index.js",
  output: {
    filename: "bundle.node.min.js",
    path: path.resolve(__dirname, "dist"),
    libraryTarget: "commonjs2"
  },
  module: {
    rules: [
      {
        test: /\.js$/,
        exclude: /(node_modules|bower_components)/,
        use: {
          loader: "babel-loader",
          options: {
            presets: ["stage-3"]
          }
        }
      }
    ]
  },
  plugins: [new UglifyJsPlugin()]
};

const browserConfig = {
  entry: "./index.js",
  output: {
    filename: "bundle.min.js",
    path: path.resolve(__dirname, "dist"),
    library: "kyber",
    libraryTarget: "umd"
  },
  module: {
    rules: [
      {
        test: /\.js$/,
        exclude: /(node_modules|bower_components)/,
        use: {
          loader: "babel-loader",
          options: {
            presets: ["stage-3"]
          }
        }
      }
    ]
  },
  plugins: [new UglifyJsPlugin()]
};

module.exports = [nodeConfig, browserConfig];
