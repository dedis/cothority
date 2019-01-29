const path = require("path");
const UglifyJsPlugin = require("uglifyjs-webpack-plugin");

module.exports = {
    entry: "./src/index.ts",
    output: {
      filename: "bundle.min.js",
      path: path.resolve(__dirname, "dist"),
      library: "cothority",
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
              presets: [
                ["env", { targets: { browsers: [">1%"] }, useBuiltIns: true }]
              ],
              plugins: [require("babel-plugin-transform-object-rest-spread")]
            }
          }
        },
        {
            test: /\.ts$/,
            exclude: /node_modules/,
            use: [
                {
                    loader: 'babel-loader',
                    options: {
                        presets: ['env'],
                    }
                },
                "ts-loader",
            ]
        }
      ]
    },
    resolve: {
        extensions: ['.ts', '.js'],
    },
    plugins: [
      new UglifyJsPlugin({
        uglifyOptions: {
          mangle: {
            safari10: true
          }
        }
      })
    ]
  };
