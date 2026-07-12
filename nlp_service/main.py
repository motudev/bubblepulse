from fastapi import FastAPI
from pydantic import BaseModel
import spacy

nlp = spacy.load("en_core_web_sm")
app = FastAPI()


class ParseRequest(BaseModel):
    text: str


def extract_action_objects(doc) -> list[str]:
    token_to_chunk = {t.i: chunk for chunk in doc.noun_chunks for t in chunk}
    topics = []
    seen = set()

    for token in doc:
        if token.pos_ != "VERB":
            continue

        objects = []
        for child in token.children:
            if child.dep_ == "dobj":
                objects.append(child)
            elif child.dep_ == "prep":
                for grandchild in child.children:
                    if grandchild.dep_ == "pobj":
                        objects.append(grandchild)

        for obj in objects:
            chunk = token_to_chunk.get(obj.i)
            if chunk:
                chunk_text = " ".join(t.text for t in chunk if t.pos_ != "DET").strip()
            else:
                chunk_text = obj.text.strip()

            if not chunk_text:
                continue

            phrase = f"{token.lemma_} {chunk_text}".lower()
            if phrase not in seen:
                seen.add(phrase)
                topics.append(phrase)

    return topics


@app.post("/parse")
def parse(req: ParseRequest):
    doc = nlp(req.text)
    return {"noun_phrases": extract_action_objects(doc)}


@app.get("/health")
def health():
    return {"status": "ok"}
