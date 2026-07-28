import { Parser } from 'n3'
import {
  fetchHubJson,
  formatOntologyLabel,
  localName,
  OntologyGraph,
  readRdfList,
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
  /** Exact datatype IRI when the shape declares a non-XSD datatype — the
   *  emitted literal must carry it verbatim to conform to the library. */
  datatypeIri?: string
  /** Enumerated allowed lexical values (sh:in). */
  options?: readonly string[]
  /** IRI-valued leaf without a target class (sh:nodeKind sh:IRI). */
  iri?: boolean
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

type LeafConstraint = Pick<ShapeProperty, 'datatype' | 'datatypeIri' | 'options' | 'iri' | 'classRef'>

/** Resolves one constraint node (a property shape or an sh:or / sh:xone
 *  branch) to an authorable input: an sh:in enumeration, a typed literal, an
 *  object reference (sh:class / sh:node), or a bare IRI (sh:nodeKind sh:IRI). */
function leafConstraint(graph: OntologyGraph, node: string): LeafConstraint | null {
  const declaredDatatype = graph.first(node, `${SH}datatype`)
  const datatype = XSD_TO_COMPACT[declaredDatatype] ?? (declaredDatatype ? 'xsd:string' : undefined)
  const datatypeIri = declaredDatatype && !XSD_TO_COMPACT[declaredDatatype] ? declaredDatatype : undefined

  const inList = graph.first(node, `${SH}in`)
  if (inList) {
    const options = readRdfList(graph, inList).filter(Boolean)
    if (options.length) {
      return { options, datatype: datatype ?? 'xsd:string', ...(datatypeIri ? { datatypeIri } : {}) }
    }
  }
  if (datatype) return { datatype, ...(datatypeIri ? { datatypeIri } : {}) }

  let classRef = graph.first(node, `${SH}class`)
  if (!classRef) {
    const nodeShape = graph.first(node, `${SH}node`)
    if (nodeShape) classRef = graph.first(nodeShape, `${SH}targetClass`)
  }
  if (classRef) return { classRef }

  if (graph.first(node, `${SH}nodeKind`) === `${SH}IRI`) return { iri: true }
  return null
}

function parseProperty(graph: OntologyGraph, propertyNode: string): ShapeProperty | null {
  const path = graph.first(propertyNode, `${SH}path`)
  // Sequence/inverse paths arrive as blank nodes — only direct predicate
  // paths are authorable form inputs.
  if (!path?.includes('://')) return null

  let constraint = leafConstraint(graph, propertyNode)
  if (!constraint) {
    // sh:or / sh:xone alternatives: the first authorable branch drives the input.
    for (const alternatives of [`${SH}or`, `${SH}xone`]) {
      const head = graph.first(propertyNode, alternatives)
      if (!head) continue
      for (const branch of readRdfList(graph, head)) {
        constraint = leafConstraint(graph, branch)
        if (constraint) break
      }
      if (constraint) break
    }
  }
  if (!constraint) return null

  const minCount = graph.firstNumber(propertyNode, `${SH}minCount`) ?? 0
  const maxCount = graph.firstNumber(propertyNode, `${SH}maxCount`)
  return {
    path,
    label: graph.first(propertyNode, `${SH}name`) || formatOntologyLabel(localName(path)),
    ...constraint,
    required: minCount >= 1,
    multiple: maxCount === undefined || maxCount > 1,
  }
}

function parseLibrary(graph: OntologyGraph, library: string): ShapeClass[] {
  const classes: ShapeClass[] = []
  for (const shape of graph.subjectsOfType(`${SH}NodeShape`)) {
    const target = graph.first(shape, `${SH}targetClass`)
    if (!target) continue
    // A class stays pickable with zero authorable properties — marker shapes
    // (e.g. gx:RegistrationNumber, everything under sh:ignoredProperties)
    // are still typed objects other shapes reference via sh:node.
    const properties = graph
      .values(shape, `${SH}property`)
      .map((node) => parseProperty(graph, node))
      .filter((property): property is ShapeProperty => property !== null)
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
