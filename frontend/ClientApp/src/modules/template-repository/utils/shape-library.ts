import { Parser } from 'n3'
import {
  fetchHubJson,
  formatOntologyLabel,
  localName,
  OntologyGraph,
} from '@template-repository/utils/ontology-domain-fields'
import type { XsdDatatype } from '@/models/dcs-jsonld'

/**
 * Authorable classes from the Semantic Hub's registered SHACL libraries: any
 * sh:NodeShape with an sh:targetClass becomes a class a Template Creator can
 * click into a document's dcs:contractData as a typed domain object. The
 * canonical DCS shapes and the clause catalog describe the document envelope
 * itself, not domain data, so they are excluded — everything else registered
 * under kind "shapes" participates.
 */

const SH = 'http://www.w3.org/ns/shacl#'
const XSD = 'http://www.w3.org/2001/XMLSchema#'

/** Hub entries that shape the document envelope, not domain data. */
const ENVELOPE_SCHEMA_NAMES = new Set(['facis-dcs', 'clause-catalog'])

export interface ShapeProperty {
  /** The predicate IRI instances carry (absolute). */
  path: string
  label: string
  /** Compact xsd datatype for a literal-valued property. */
  datatype?: XsdDatatype
  /** Target class IRI for an object-valued property (sh:class / sh:node). */
  classRef?: string
  required: boolean
  multiple: boolean
}

export interface ShapeClass {
  /** The class IRI instances are typed with (absolute). */
  iri: string
  shapeIri: string
  label: string
  library: string
  properties: ShapeProperty[]
}

const XSD_TO_COMPACT: Record<string, XsdDatatype> = {
  [`${XSD}string`]: 'xsd:string',
  [`${XSD}decimal`]: 'xsd:decimal',
  [`${XSD}double`]: 'xsd:decimal',
  [`${XSD}float`]: 'xsd:decimal',
  [`${XSD}integer`]: 'xsd:integer',
  [`${XSD}int`]: 'xsd:integer',
  [`${XSD}long`]: 'xsd:integer',
  [`${XSD}boolean`]: 'xsd:boolean',
  [`${XSD}date`]: 'xsd:date',
  [`${XSD}dateTime`]: 'xsd:dateTime',
}

function parseProperty(
  graph: OntologyGraph,
  propertyNode: string,
  shapeByTarget: Map<string, string>,
): ShapeProperty | null {
  const path = graph.first(propertyNode, `${SH}path`)
  // Sequence/inverse paths arrive as blank nodes — only direct predicate
  // paths are authorable form inputs.
  if (!path?.includes('://')) return null

  const datatypeIri = graph.first(propertyNode, `${SH}datatype`)
  let classRef = graph.first(propertyNode, `${SH}class`)
  if (!classRef) {
    const nodeShape = graph.first(propertyNode, `${SH}node`)
    if (nodeShape) {
      classRef = graph.first(nodeShape, `${SH}targetClass`) || (shapeByTarget.get(nodeShape) ?? '')
    }
  }
  const datatype = XSD_TO_COMPACT[datatypeIri]
  if (!datatype && !classRef) return null

  const minCount = graph.firstNumber(propertyNode, `${SH}minCount`) ?? 0
  const maxCount = graph.firstNumber(propertyNode, `${SH}maxCount`)
  return {
    path,
    label: graph.first(propertyNode, `${SH}name`) || formatOntologyLabel(localName(path)),
    ...(datatype ? { datatype } : {}),
    ...(classRef ? { classRef } : {}),
    required: minCount >= 1,
    multiple: maxCount === undefined || maxCount > 1,
  }
}

function parseLibrary(graph: OntologyGraph, library: string): ShapeClass[] {
  // sh:node references name a NodeShape; instances are typed with its
  // target class — resolve shape IRI -> target class up front.
  const shapeByTarget = new Map<string, string>()
  for (const shape of graph.subjectsOfType(`${SH}NodeShape`)) {
    const target = graph.first(shape, `${SH}targetClass`)
    if (target) shapeByTarget.set(shape, target)
  }

  const classes: ShapeClass[] = []
  for (const shape of graph.subjectsOfType(`${SH}NodeShape`)) {
    const target = graph.first(shape, `${SH}targetClass`)
    if (!target) continue
    const properties = graph
      .values(shape, `${SH}property`)
      .map((node) => parseProperty(graph, node, shapeByTarget))
      .filter((property): property is ShapeProperty => property !== null)
    if (!properties.length) continue
    classes.push({
      iri: target,
      shapeIri: shape,
      label: formatOntologyLabel(localName(target)),
      library,
      properties,
    })
  }
  return classes
}

interface SchemaListEntry {
  name: string
  kind: string
  active_version: number
}

/** Loads every authorable class from the hub's ACTIVE registered shape
 *  libraries. Failures are hard errors — a missing hub is a deployment
 *  fault, not an empty palette. */
export async function loadShapeLibraries(): Promise<ShapeClass[]> {
  const entries = await fetchHubJson<SchemaListEntry[]>('/api/semantic/schema/list')
  const libraries = entries.filter(
    (entry) => entry.kind === 'shapes' && entry.active_version > 0 && !ENVELOPE_SCHEMA_NAMES.has(entry.name),
  )
  const classes: ShapeClass[] = []
  for (const library of libraries) {
    const body = await fetchHubJson<{ content: string }>(`/api/semantic/shapes/${encodeURIComponent(library.name)}`)
    const graph = new OntologyGraph(new Parser().parse(body.content))
    classes.push(...parseLibrary(graph, library.name))
  }
  return classes.sort((a, b) => a.label.localeCompare(b.label))
}
