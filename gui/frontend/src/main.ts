import "./app.css";
import { mount } from "svelte";
import App from "./App.svelte";
import { installErrorLog } from "./lib/report/errorLog";

installErrorLog();

const target = document.getElementById("app");
if (!target) throw new Error("missing #app mount point");

export default mount(App, { target });
