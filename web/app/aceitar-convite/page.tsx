import {Suspense} from "react";
import {AuthForm} from "../../components/auth-form";
export default function Page(){return <Suspense fallback={<p>Carregando…</p>}><AuthForm mode="accept"/></Suspense>}
