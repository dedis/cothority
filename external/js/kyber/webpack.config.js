const path = require("path");

module.exports = {
  entry: "./src/index.ts",
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
        exclude: /node_modules/,
        use: {
          loader: "babel-loader",
          options: {
            presets: ["env"]
          }
        }
      },
      {
        test: /\.tsx?$/,
        exclude: /(node_modules|bower_components)/,
        use: [
          {
            loader: "babel-loader",
            options: {
              presets: ["env"]
            }
          },
          "ts-loader"
        ]
      }
    ]
  },
  resolve: {
    extensions: [".tsx", ".ts", ".js"]
  }
};
