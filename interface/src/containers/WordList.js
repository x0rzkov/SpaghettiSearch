import React, { Component } from 'react';
import { Button, Container, Row, Col, Pagination, PaginationItem, PaginationLink } from 'reactstrap';
import history from '../utils/history'
import '../styles/WordList.css'
const axios = require('axios');
const config = require('../config/server');

class WordList extends Component {
  constructor() {
    super();
    this.state = {
			data: "",
			maxRow: 15,
			maxCol: 5,
			paginationWinSize: 5,
			currPage: 1,
			currPageD: [],
			currPre: "A",
			termList: [],
		};
  }

  componentDidMount(props) {
		let currO = this;
    axios.get(config.address+'wordlist/a').then(function(res) {
			let temp = []
			for(let i in res.data) {
				if(i >= currO.state.maxRow * currO.state.maxCol) break;
				temp.push(res.data[i]);
			}
      currO.setState({ data: res.data, currPageD: temp });
    });
  }

  handleSubmit = () => {
    console.log(this.state.termList.join(" "))
    history.push(
      '/query',
      {query: this.state.termList.join(" ")}
    );
    document.location.reload(true)
  }

	updateCurrData(pre) {
		let currO = this;
    axios.get(config.address+'wordlist/' + pre.toLowerCase()).then(function(res) {
			let temp = []
			for(let i in res.data) {
				if(i >= currO.state.maxRow * currO.state.maxCol) break;
				temp.push(res.data[i]);
			}
      currO.setState({ data: res.data, currPageD: temp, currPage: 1, currPre: pre });
    });
	}

	addTermList(term) {
		if(!this.state.termList.includes(term)) {
			this.setState({ termList: this.state.termList.concat([term]) });
		} else {
			this.setState({ termList: this.state.termList.filter((x) => { return x !== term }) });
		}
	}

	getCurrPageArr() {
		let results = [];
		for(let i=0; i < this.state.maxRow; i++) {
			let children = [];
			for(let j=0; j < this.state.maxCol; j++) {
				let thisElement = this.state.currPageD[i*this.state.maxCol + j];
				children.push(
					<Col key={j}>
						<Button color="link" onClick={this.addTermList.bind(this, thisElement)}>
							{thisElement}
						</Button>
					</Col>
				);
			}
			results.push(<Row key={i}>{children}</Row>);
		}
		return results;
	}

	updateCurrPageD(page) {
		let newD = [];
		let idx = (page-1) * this.state.maxRow * this.state.maxCol;
		for(let i=0; i < this.state.maxCol * this.state.maxRow; i++) {
			if(idx + i >= this.state.data.length) break;
			newD.push(this.state.data[idx+i]);
		}
		this.setState({ currPageD: newD, currPage: page });
	}

	getPagination() {
		let inner = [];
		let total = Math.ceil(this.state.data.length / (this.state.maxRow * this.state.maxCol));
		if(total <= 5) {
			for(let i=0; i < total; i++) {
				inner.push(
					<PaginationItem key={i} active={this.state.currPage === i + 1}>
						<PaginationLink href="#" onClick={this.updateCurrPageD.bind(this, i+1)}>
							{i+1}
						</PaginationLink>
					</PaginationItem>
				);
			}
		} else {
			let start = this.state.currPage - Math.floor(this.state.paginationWinSize / 2);
			let end = this.state.currPage + Math.floor(this.state.paginationWinSize / 2);
			if(start < 1) {
				end += 1 - start;
				start = 1;
			}
			if(end > total) {
				start -= end - total;
				end = total;
			}
			for(let i = start; i <= end; i++) {
				inner.push(
					<PaginationItem key={i} active={this.state.currPage === i}>
						<PaginationLink href="#" onClick={this.updateCurrPageD.bind(this, i)}>
							{i}
						</PaginationLink>
					</PaginationItem>
				);
			}
		}
		return (
			<Pagination aria-label="Page navigation example" style={{justifyContent: "right"}}>
				<PaginationItem disabled={this.state.currPage === 1}>
					<PaginationLink first href="#" onClick={this.updateCurrPageD.bind(this, 1)}/>
				</PaginationItem>
				<PaginationItem disabled={this.state.currPage === 1}>
					<PaginationLink previous href="#" onClick={this.updateCurrPageD.bind(this, this.state.currPage - 1)}/>
				</PaginationItem>
				{inner}
				<PaginationItem disabled={this.state.currPage === total}>
					<PaginationLink next href="#" onClick={this.updateCurrPageD.bind(this, this.state.currPage + 1)}/>
				</PaginationItem>
				<PaginationItem disabled={this.state.currPage === total}>
					<PaginationLink last href="#" onClick={this.updateCurrPageD.bind(this, total)}/>
				</PaginationItem>
			</Pagination>
		);
	}

  render() {
		const preList = [
			"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		];
		let items = [];
			for(let i in preList[0]) {
				items.push(
						<Button key={i} block color={this.state.currPre === preList[0][i] ? "secondary" : "info"}
						disabled={this.state.currPre === preList[0][i]}
						onClick={this.updateCurrData.bind(this, preList[0][i])}>
							{preList[0][i]}
						</Button>
				);
			}

    return (
			<div className="WordList">
				<br/>
				<br/>
        <div className='alphabet'>
			    {items}
        </div>

					<br/>
					<br/>

			    {this.getCurrPageArr()}
					<br/>
					<br/>
          <Container>
					<Row>
						<Col>Selected Terms: <br/>{"[" + this.state.termList.join(", ") + "]"}</Col>
						<Col>
							<Button color="primary" onClick={this.handleSubmit} block>Search</Button>
						</Col>
						<Col>{this.getPagination()}</Col>
					</Row>
					<br/>
			  </Container>
			</div>
    );
  }
}

export default WordList;
