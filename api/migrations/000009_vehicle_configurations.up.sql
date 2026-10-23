ALTER TABLE axle_templates ADD COLUMN category text NOT NULL DEFAULT 'Outros';
UPDATE axle_templates SET category = CASE id WHEN 'tractor-6x2' THEN 'Cavalo mecânico' WHEN 'trailer-3' THEN 'Carreta' ELSE category END;
INSERT INTO axle_templates(id,category,name,positions) VALUES
 ('tractor-4x2','Cavalo mecânico','Cavalo mecânico · 4x2','["1EE", "1DE", "2EE", "2EI", "2DI", "2DE", "STEP1"]'::jsonb),
 ('tractor-6x4','Cavalo mecânico','Cavalo mecânico · 6x4','["1EE", "1DE", "2EE", "2EI", "2DI", "2DE", "3EE", "3EI", "3DI", "3DE", "STEP1"]'::jsonb),
 ('tractor-8x2','Cavalo mecânico','Cavalo mecânico · 8x2','["1EE", "1DE", "2EE", "2DE", "3EE", "3EI", "3DI", "3DE", "4EE", "4EI", "4DI", "4DE", "STEP1"]'::jsonb),
 ('trailer-2','Carreta','Carreta · 2 eixos','["1EE", "1EI", "1DI", "1DE", "2EE", "2EI", "2DI", "2DE", "STEP1", "STEP2"]'::jsonb),
 ('trailer-4','Carreta','Carreta · 4 eixos','["1EE", "1EI", "1DI", "1DE", "2EE", "2EI", "2DI", "2DE", "3EE", "3EI", "3DI", "3DE", "4EE", "4EI", "4DI", "4DE", "STEP1", "STEP2"]'::jsonb),
 ('truck-4x2','Caminhão / truck','Caminhão / truck · Toco — 2 eixos','["1EE", "1DE", "2EE", "2EI", "2DI", "2DE", "STEP1"]'::jsonb),
 ('truck-6x2','Caminhão / truck','Caminhão / truck · Truck — 3 eixos','["1EE", "1DE", "2EE", "2EI", "2DI", "2DE", "3EE", "3EI", "3DI", "3DE", "STEP1"]'::jsonb),
 ('truck-6x4','Caminhão / truck','Caminhão / truck · Traçado — 3 eixos','["1EE", "1DE", "2EE", "2EI", "2DI", "2DE", "3EE", "3EI", "3DI", "3DE", "STEP1"]'::jsonb),
 ('truck-8x2','Caminhão / truck','Caminhão / truck · Bitruck — 4 eixos','["1EE", "1DE", "2EE", "2DE", "3EE", "3EI", "3DI", "3DE", "4EE", "4EI", "4DI", "4DE", "STEP1"]'::jsonb),
 ('vuc-single','VUC','VUC · 2 eixos, rodado simples','["1EE", "1DE", "2EE", "2DE", "STEP1"]'::jsonb),
 ('vuc-dual','VUC','VUC · 2 eixos, rodado duplo traseiro','["1EE", "1DE", "2EE", "2EI", "2DI", "2DE", "STEP1"]'::jsonb),
 ('light-truck-single','3/4','3/4 · 2 eixos, rodado simples','["1EE", "1DE", "2EE", "2DE", "STEP1"]'::jsonb),
 ('light-truck-dual','3/4','3/4 · 2 eixos, rodado duplo traseiro','["1EE", "1DE", "2EE", "2EI", "2DI", "2DE", "STEP1"]'::jsonb),
 ('van-single','Van','Van · 2 eixos, rodado simples','["1EE", "1DE", "2EE", "2DE", "STEP1"]'::jsonb),
 ('van-dual','Van','Van · 2 eixos, rodado duplo traseiro','["1EE", "1DE", "2EE", "2EI", "2DI", "2DE", "STEP1"]'::jsonb),
 ('utility-single','Utilitário','Utilitário · 2 eixos, rodado simples','["1EE", "1DE", "2EE", "2DE", "STEP1"]'::jsonb);
