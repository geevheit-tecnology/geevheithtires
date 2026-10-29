CREATE TABLE support_tickets (
 id uuid PRIMARY KEY,company_id uuid NOT NULL REFERENCES companies(id),created_by uuid NOT NULL REFERENCES users(id),
 subject text NOT NULL CHECK(length(subject) BETWEEN 5 AND 160),description text NOT NULL CHECK(length(description) BETWEEN 10 AND 5000),
 category text NOT NULL CHECK(category IN ('bug','error','question','suggestion')),
 module text NOT NULL CHECK(length(module) BETWEEN 2 AND 80),page_path text NOT NULL DEFAULT '' CHECK(length(page_path)<=200),
 status text NOT NULL DEFAULT 'open' CHECK(status IN ('open','triage','in_progress','resolved','closed')),
 priority text NOT NULL DEFAULT 'normal' CHECK(priority IN ('low','normal','high','critical')),
 version bigint NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(company_id,id)
);
CREATE INDEX support_ticket_queue ON support_tickets(status,updated_at DESC,id DESC);
CREATE INDEX support_ticket_tenant ON support_tickets(company_id,created_at DESC,id DESC);
CREATE TABLE support_messages (
 id uuid PRIMARY KEY,company_id uuid NOT NULL,ticket_id uuid NOT NULL,author_id uuid NOT NULL REFERENCES users(id),
 body text NOT NULL CHECK(length(body) BETWEEN 2 AND 5000),staff boolean NOT NULL DEFAULT false,internal boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(),FOREIGN KEY(company_id,ticket_id) REFERENCES support_tickets(company_id,id)
);
CREATE INDEX support_messages_thread ON support_messages(ticket_id,created_at,id);
ALTER TABLE support_tickets ENABLE ROW LEVEL SECURITY;
ALTER TABLE support_tickets FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant ON support_tickets USING(company_id=nullif(current_setting('app.company_id',true),'')::uuid OR current_user<>'geevheit_runtime') WITH CHECK(company_id=nullif(current_setting('app.company_id',true),'')::uuid OR current_user<>'geevheit_runtime');
ALTER TABLE support_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE support_messages FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant ON support_messages USING((company_id=nullif(current_setting('app.company_id',true),'')::uuid AND NOT internal) OR current_user<>'geevheit_runtime') WITH CHECK((company_id=nullif(current_setting('app.company_id',true),'')::uuid AND NOT internal AND NOT staff) OR current_user<>'geevheit_runtime');
GRANT SELECT,INSERT ON support_tickets,support_messages TO geevheit_runtime;

CREATE FUNCTION platform_tickets(actor uuid,term text,state text,urgency text,cursor_id text) RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE result jsonb;
BEGIN
 IF NOT platform_allowed(actor) THEN RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501'; END IF;
 WITH page AS (SELECT t.*,c.name company_name,u.name reporter_name FROM support_tickets t JOIN companies c ON c.id=t.company_id JOIN users u ON u.id=t.created_by WHERE (term='' OR t.subject ILIKE '%'||term||'%' OR c.name ILIKE '%'||term||'%') AND (state='' OR t.status=state) AND (urgency='' OR t.priority=urgency) AND (cursor_id='' OR t.id<cursor_id::uuid) ORDER BY t.id DESC LIMIT 51),visible AS (SELECT * FROM page ORDER BY id DESC LIMIT 50)
 SELECT jsonb_build_object('data',coalesce((SELECT jsonb_agg(to_jsonb(v) ORDER BY id DESC) FROM visible v),'[]'),'next_cursor',CASE WHEN (SELECT count(*) FROM page)>50 THEN (SELECT id::text FROM visible ORDER BY id LIMIT 1) ELSE '' END,
 'counts',jsonb_build_object('open',(SELECT count(*) FROM support_tickets WHERE status IN ('open','triage')),'in_progress',(SELECT count(*) FROM support_tickets WHERE status='in_progress'),'critical',(SELECT count(*) FROM support_tickets WHERE priority='critical' AND status NOT IN ('resolved','closed')))) INTO result;
 RETURN result;
END $$;
CREATE FUNCTION platform_ticket_detail(actor uuid,target_id uuid) RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE result jsonb;
BEGIN
 IF NOT platform_allowed(actor) THEN RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501'; END IF;
 SELECT jsonb_build_object('ticket',to_jsonb(t)||jsonb_build_object('company_name',c.name,'reporter_name',u.name),
 'messages',coalesce((SELECT jsonb_agg(to_jsonb(m)||jsonb_build_object('author_name',a.name) ORDER BY m.created_at,m.id) FROM support_messages m JOIN users a ON a.id=m.author_id WHERE m.ticket_id=t.id),'[]')) INTO result FROM support_tickets t JOIN companies c ON c.id=t.company_id JOIN users u ON u.id=t.created_by WHERE t.id=target_id;
 IF result IS NULL THEN RAISE EXCEPTION 'Unknown ticket' USING ERRCODE='P0002'; END IF;RETURN result;
END $$;
CREATE FUNCTION platform_update_ticket(actor uuid,target_id uuid,expected_version bigint,new_status text,new_priority text,justification text,reply text,is_internal boolean) RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
DECLARE tenant uuid;
BEGIN
 IF NOT platform_allowed(actor) THEN RAISE EXCEPTION 'Forbidden' USING ERRCODE='42501'; END IF;
 IF new_status NOT IN ('open','triage','in_progress','resolved','closed') OR new_priority NOT IN ('low','normal','high','critical') OR length(trim(justification)) NOT BETWEEN 5 AND 500 OR length(reply)>5000 OR (reply<>'' AND length(trim(reply))<2) THEN RAISE EXCEPTION 'Invalid ticket update' USING ERRCODE='22023'; END IF;
 UPDATE support_tickets SET status=new_status,priority=new_priority,updated_at=now(),version=version+1 WHERE id=target_id AND version=expected_version RETURNING company_id INTO tenant;
 IF NOT FOUND THEN RAISE EXCEPTION 'Ticket changed; refresh' USING ERRCODE='40001'; END IF;
 IF reply<>'' THEN INSERT INTO support_messages(id,company_id,ticket_id,author_id,body,staff,internal) VALUES(gen_random_uuid(),tenant,target_id,actor,reply,true,is_internal); END IF;
 INSERT INTO platform_audit(actor_id,action,target,reason,data) VALUES(actor,'update_ticket',target_id::text,justification,jsonb_build_object('status',new_status,'priority',new_priority,'internal',is_internal));
END $$;
REVOKE ALL ON FUNCTION platform_tickets(uuid,text,text,text,text),platform_ticket_detail(uuid,uuid),platform_update_ticket(uuid,uuid,bigint,text,text,text,text,boolean) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform_tickets(uuid,text,text,text,text),platform_ticket_detail(uuid,uuid),platform_update_ticket(uuid,uuid,bigint,text,text,text,text,boolean) TO geevheit_runtime;
