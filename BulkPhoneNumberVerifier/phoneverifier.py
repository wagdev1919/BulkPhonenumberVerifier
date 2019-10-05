#!/usr/bin/env python3
# -*- coding:utf-8 -*-
#
# @name   : PhoneInfoga - Phone numbers OSINT tool
# @url    : https://github.com/sundowndev
# @author : Raphael Cerveaux (sundowndev)

# dependencies
import sys
import signal

import requests
from fake_useragent import UserAgent
from urllib.request import Request, urlopen
from urllib.error import HTTPError
from bs4 import BeautifulSoup
from numpy import loadtxt
#from robobrowser import RoboBrowser
#from selenium import webdriver #Firefox
from multiprocessing import Process, Lock, Manager, Value
import multiprocessing
import time
import random
import csv
import psutil

# lib
from lib.args import args,parser
from lib.output import *
from lib.format import *
from lib.logger import Logger
#from lib.googlesearch import closeBrowser
# scanners
from scanners import numverify
from scanners import localscan

MAX_NUMBERS_PER_THREAD = 50
MAX_THREAD_LIMIT = 30

# update proxies function
def update_proxies(max_limit, proxies, phonenum_list, fail_proxies, used_proxies, lock):
    print("update_proxies began!")
    # Retrieve latest proxies
    ua = UserAgent() # From here we generate a random user agent
    while 1:
        proxies_req = Request('https://www.sslproxies.org/')
        proxies_req.add_header('User-Agent', ua.random)
        try:
            proxies_doc = urlopen(proxies_req).read().decode('utf8')
        except HTTPError as e:
            time.sleep(10)
            continue

        soup = BeautifulSoup(proxies_doc, 'html.parser')
        proxies_table = soup.find(id='proxylisttable')

        # Save proxies in the array
        count = 0
        for row in proxies_table.tbody.find_all('tr'):
            #if row.find_all('td')[4].string == "anonymous":
            new_proxy = {
                'ip':   row.find_all('td')[0].string,
                'port': row.find_all('td')[1].string,
                'country': row.find_all('td')[3].string,
            }
            new_proxy_addr = "{}:{}".format(new_proxy['ip'], new_proxy['port'])
            new_one = True
            #lock.acquire()
            if new_proxy_addr in proxies or new_proxy_addr in fail_proxies or new_proxy_addr in used_proxies:
                new_one = False
            #lock.release()
            
            if not new_one:
                continue
            lock.acquire()
            if len(proxies) < max_limit:
                proxies.append(new_proxy_addr)
                count += 1
                print("%s:%s: %s" %(new_proxy['ip'], new_proxy['port'], new_proxy['country']))
            lock.release()
        print("{}proxies appended ### proxy count:{}".format(count, len(proxies)))
        time.sleep(10)
    print("There is no phone numbers any more.")

# end - update proxies function

def scanNumber(InputNumber, proxy):
    result = 0
    invalid_data = {'valid':False, 'country_name':'NONE', 'location':'NONE', 'carrier':'NONE', 'line_type':'NONE'}
    try:
        number = localscan.scan(InputNumber, False)
    except Exception as e:
        data = invalid_data
        error("Invalid phone number format:{}".format(InputNumber))

    if not number:
        data = invalid_data
    else:
        result, data = numverify.scan(number['default'], proxy)
        
    #ovh.scan(number['local'], number['countryIsoCode'])
    #recon.scan(number)
    #osintScan(number)
    return result, data

def scanNumbersWithProxy(proxy, phonenum_list, result_list, bunch_size, fail_proxies, used_proxies, lock):
    print("Starting Thread:{}".format(multiprocessing.current_process().name))
    for i in range(bunch_size.value):
        phonenum = ""
        lock.acquire()
        if len(phonenum_list) > 0:
            phonenum = phonenum_list.pop()
        lock.release()
        if phonenum == "":
            break
        result, data = scanNumber(phonenum, proxy)
        if result == -1:
            print("Scanning Failed-{} is not correct format.".format(phonenum))
            lock.acquire()
            result_list.append({'phone_number':phonenum, 'valid':False, 'Country':'NONE', 'Location':'NONE', 'Carrier':'NONE', 'Line type':'NONE'})
            lock.release()
        elif result == -2:
            warn("Scanning Failed-connection error:{},{}".format(multiprocessing.current_process().name, proxy.get('https')))
            lock.acquire()
            phonenum_list.append(phonenum)#repeat process later
            fail_proxies.append(proxy.get('https'))
            lock.release()
            break
        elif result == 0:
            print("%s#Scanning Finished--------------%s:%s" % (multiprocessing.current_process().name, phonenum, data['valid']))
            res = {'phone_number':phonenum, 'valid':data['valid'], 'Country':data['country_name'], 'Location':data["location"], 'Carrier':data["carrier"], 'Line type':data["line_type"]}
            lock.acquire()
            result_list.append(res)
            lock.release()

        #time.sleep(1)
    lock.acquire()
    if proxy.get('https') in used_proxies:
        used_proxies.remove(proxy.get('https'))
    lock.release()
    print("Thread:{} Finished!".format(multiprocessing.current_process().name))

def main():
    # Ensure the usage of Python3
    if sys.version_info[0] < 3:
        print("(!) Please run the tool using Python 3")
        sys.exit()

    # If any param is passed, execute help command
    if not len(sys.argv) > 1:
        parser.print_help()
        sys.exit()

    if args.output:
        sys.stdout = Logger()

    #reading phone numbers
    plist = loadtxt(args.input, dtype=str).tolist()
    total_cnt = len(plist)

    manager = Manager()
    proxies = manager.list()
    phonenum_list = manager.list()
    result_list = manager.list()
    procs = []
    fail_proxies = manager.list()
    used_proxies = manager.list()
    
    for n in plist: 
        phonenum_list.append(n)

    print("Inputed phonenums:" + str(total_cnt))
    start = time.time()
    
    result_file = open('result.csv', 'w')
    csvwriter = csv.writer(result_file)
    csvwriter.writerow(["phone_number", 'valid', 'Country', 'Location', 'Carrier', 'Line type'])

    lock = Lock()
    prox_up_t = Process(target=update_proxies, args = (MAX_THREAD_LIMIT * 3, proxies, phonenum_list, fail_proxies, used_proxies, lock, ))
    prox_up_t.start()

    prev_result_cnt = 0
    result_cnt = 0
    update_time = time.time()
    while 1:
        lock.acquire()
        for res in result_list:
            csvwriter.writerow(res.values())
            result_list.remove(res)
            result_cnt += 1
        lock.release()

        if result_cnt > prev_result_cnt:
            result_file.flush()
            update_time = time.time()
            prev_result_cnt = result_cnt
            plus("{} phone numbers has been processed".format(result_cnt))
            
        else:#long time failed during getting proxies
            hours, rem = divmod(time.time()-update_time, 3600)
            minutes, seconds = divmod(rem, 60)
            if minutes > 3:#over 3 min
                lock.acquire()
                for x in fail_proxies:
                    fail_proxies.remove(x)
                lock.release()

        if result_cnt >= total_cnt:
            prox_up_t.terminate()
            break

        #lock.acquire()
        for p in procs:
            if not p.is_alive():
                p.terminate()
                procs.remove(p)

        remains = total_cnt - result_cnt
        #lock.release()
        if len(phonenum_list) > 0 and len(procs) < min(MAX_THREAD_LIMIT, remains):
            proxyaddr = ""
            lock.acquire()
            if len(proxies) > 0:
                proxyaddr = proxies.pop()
                used_proxies.append(proxyaddr)
            lock.release()
            if proxyaddr != "":
                bunch_size = Value('i', min(MAX_NUMBERS_PER_THREAD, max(1, int(remains / MAX_THREAD_LIMIT))))
                t = Process(target=scanNumbersWithProxy, args=({'https': proxyaddr}, phonenum_list, result_list, bunch_size, fail_proxies, used_proxies, lock, ))
                t.start()
                procs.append(t)
        else:
            time.sleep(1)

    for p in procs:
        p.terminate()
    end = time.time()
    hours, rem = divmod(end-start, 3600)
    minutes, seconds = divmod(rem, 60)
    
    result_file.close()
    
    print("Ellapsed time: {:0>2}:{:0>2}:{:05.2f}".format(int(hours),int(minutes),seconds))
    
    if args.output:
        args.output.close()
        

def signal_handler(signal, frame):
    print('\n[-] You pressed Ctrl+C! Exiting.')

    sys.exit()


if __name__ == '__main__':
    signal.signal(signal.SIGINT, signal_handler)
    main()
