// Bundles cssedit.js into static/js/cssedit.bundle.js.
import * as esbuild from "esbuild";

await esbuild.build({
	entryPoints: ["cssedit.js"],
	bundle: true,
	minify: true,
	format: "iife",
	globalName: "WFCSSEditor",
	target: "es2018",
	outfile: "../static/js/cssedit.bundle.js",
	banner: {
		js: "/*! WriteFreely CSS editor. Includes CodeMirror 6 (https://codemirror.net), MIT License, Copyright (C) 2018-2021 by Marijn Haverbeke <marijn@haverbeke.berlin> and others */",
	},
	logLevel: "info",
});
